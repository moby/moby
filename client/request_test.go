package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestSetHostHeader should set fake host for local communications, set real host
// for normal communications.
func TestSetHostHeader(t *testing.T) {
	testURL := "/test"
	testCases := []struct {
		host            string
		expectedHost    string
		expectedURLHost string
	}{
		{
			"unix:///var/run/docker.sock",
			"docker",
			"/var/run/docker.sock",
		},
		{
			"npipe:////./pipe/docker_engine",
			"docker",
			"//./pipe/docker_engine",
		},
		{
			"tcp://0.0.0.0:4243",
			"",
			"0.0.0.0:4243",
		},
		{
			"tcp://localhost:4243",
			"",
			"localhost:4243",
		},
	}

	for c, test := range testCases {
		hostURL, err := ParseHostURL(test.host)
		assert.NilError(t, err)

		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, testURL) {
					return nil, fmt.Errorf("Test Case #%d: Expected URL %q, got %q", c, testURL, req.URL)
				}
				if req.Host != test.expectedHost {
					return nil, fmt.Errorf("Test Case #%d: Expected host %q, got %q", c, test.expectedHost, req.Host)
				}
				if req.URL.Host != test.expectedURLHost {
					return nil, fmt.Errorf("Test Case #%d: Expected URL host %q, got %q", c, test.expectedURLHost, req.URL.Host)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(""))),
				}, nil
			}),

			proto:    hostURL.Scheme,
			addr:     hostURL.Host,
			basePath: hostURL.Path,
		}

		_, err = versionedClient{cli: client}.sendRequest(context.Background(), http.MethodGet, testURL, nil, nil, nil)
		assert.NilError(t, err)
	}
}

// TestPlainTextError tests the server returning an error in plain text for
// backwards compatibility with API versions <1.24. All other tests use
// errors returned as JSON
func TestPlainTextError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(plainTextErrorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	_, err = client.ContainerList(context.Background(), types.ContainerListOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestInfiniteError(t *testing.T) {
	infinitR := rand.New(rand.NewSource(42))
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusInternalServerError}
			resp.Header = http.Header{}
			resp.Body = io.NopCloser(infinitR)
			return resp, nil
		})),
	)
	assert.NilError(t, err)

	_, err = client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, "request returned Internal Server Error"))
}

func TestCanceledContext(t *testing.T) {
	testURL := "/test"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, req.Context().Err(), context.Canceled)

			return nil, context.Canceled
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := versionedClient{cli: client}.sendRequest(ctx, http.MethodGet, testURL, nil, nil, nil)
	assert.Equal(t, true, errdefs.IsCancelled(err))
	assert.Equal(t, true, errors.Is(err, context.Canceled))
}

func TestDeadlineExceededContext(t *testing.T) {
	testURL := "/test"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, req.Context().Err(), context.DeadlineExceeded)

			return nil, context.DeadlineExceeded
		}),
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()

	<-ctx.Done()

	_, err := versionedClient{cli: client}.sendRequest(ctx, http.MethodGet, testURL, nil, nil, nil)
	assert.Equal(t, true, errdefs.IsDeadline(err))
	assert.Equal(t, true, errors.Is(err, context.DeadlineExceeded))
}

func TestConcurrentRequests(t *testing.T) {
	var mu sync.Mutex
	reqs := make(map[string]int)

	client, err := NewClientWithOpts(
		WithAPIVersionNegotiation(),
		WithHTTPClient(newMockClient(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			reqs[r.Method+" "+r.URL.Path]++
			mu.Unlock()
			header := make(http.Header)
			header.Set("API-Version", "1.30")
			return &http.Response{
				StatusCode: 200,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader("{}")),
			}, nil
		})),
	)
	assert.NilError(t, err)

	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			_, err := client.Info(context.Background())
			assert.Check(t, err)
		}()
	}
	wg.Wait()

	assert.DeepEqual(t, reqs, map[string]int{
		"HEAD /_ping":     1,
		"GET /v1.30/info": 3,
	})
}
