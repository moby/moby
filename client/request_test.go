package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestRetryNegotiation(t *testing.T) {
	type testcase struct {
		name       string
		handlePing func() (*http.Response, error)
	}

	status := func(code int) testcase {
		return testcase{
			name: fmt.Sprintf("StatusCode=%d", code),
			handlePing: func() (*http.Response, error) {
				return &http.Response{
					StatusCode: code,
					Body:       io.NopCloser(strings.NewReader(http.StatusText(code))),
				}, nil
			},
		}
	}

	for _, tt := range []testcase{
		status(http.StatusBadGateway),         // HTTP 502
		status(http.StatusServiceUnavailable), // HTTP 503
		status(http.StatusGatewayTimeout),     // HTTP 504
		{
			name: "RequestError",
			handlePing: func() (*http.Response, error) {
				return nil, fmt.Errorf("fake request error")
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var handler func(*http.Request) (*http.Response, error)
			client, err := NewClientWithOpts(
				WithAPIVersionNegotiation(),
				WithHTTPClient(newMockClient(func(r *http.Request) (*http.Response, error) {
					t.Logf("Mock HTTP client: %s %s", r.Method, r.URL)
					return handler(r)
				})),
			)
			assert.NilError(t, err)

			handler = func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/_ping" {
					t.Errorf("unexpected request to %s %s", r.Method, r.URL)
					return nil, fmt.Errorf("unexpected request")
				}
				return tt.handlePing()
			}
			info, err := client.Info(context.Background())
			assert.Check(t, is.DeepEqual(types.Info{}, info))
			assert.Check(t, err != nil)

			// This time allow negotiation to succeed but respond to
			// the request for daemon info with an error.
			handler = func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/_ping":
					header := make(http.Header)
					header.Set("API-Version", "1.30")
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Header:     header,
						Body:       io.NopCloser(strings.NewReader("pong")),
					}, nil
				case "/v1.30/info":
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(strings.NewReader("don't feel like it today")),
					}, nil
				}
				t.Errorf("unexpected request to %s %s", r.Method, r.URL)
				return nil, fmt.Errorf("unexpected request")
			}
			info, err = client.Info(context.Background())
			assert.Check(t, is.DeepEqual(types.Info{}, info))
			assert.Check(t, is.ErrorContains(err, "don't feel like it today"))

			// Get info again, successfully this time. No version
			// negotiation should take place.
			expectedInfo := types.Info{Name: "fake-info"}
			infoJSON, err := json.Marshal(&expectedInfo)
			assert.NilError(t, err)
			handler = func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/v1.30/info" {
					header := make(http.Header)
					header.Set("Content-Type", "application/json")
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     header,
						Body:       io.NopCloser(bytes.NewReader(infoJSON)),
					}, nil
				}
				t.Errorf("unexpected request to %s %s", r.Method, r.URL)
				return nil, fmt.Errorf("unexpected request")
			}
			info, err = client.Info(context.Background())
			assert.Check(t, err)
			assert.Check(t, is.DeepEqual(info, expectedInfo))
		})
	}

	t.Run("ContextCanceled", func(t *testing.T) {
		var handler func(*http.Request) (*http.Response, error)
		client, err := NewClientWithOpts(
			WithAPIVersionNegotiation(),
			WithHTTPClient(newMockClient(func(r *http.Request) (*http.Response, error) {
				t.Logf("Mock HTTP client: %s %s", r.Method, r.URL)
				return handler(r)
			})),
		)
		assert.NilError(t, err)

		// Cancel the context while the ping request is in-flight.
		ctx, cancel := context.WithCancel(context.Background())
		handler = func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/_ping" {
				t.Errorf("unexpected request to %s %s", r.Method, r.URL)
				return nil, fmt.Errorf("unexpected request")
			}
			cancel()
			return nil, ctx.Err()
		}
		info, err := client.Info(ctx)
		assert.Check(t, is.DeepEqual(types.Info{}, info))
		assert.Check(t, is.ErrorIs(err, context.Canceled))

		// This time allow negotiation to succeed but cancel the context
		// while the info request is in-flight.
		ctx, cancel = context.WithCancel(context.Background())
		handler = func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/_ping":
				header := make(http.Header)
				header.Set("API-Version", "1.30")
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     header,
					Body:       io.NopCloser(strings.NewReader("pong")),
				}, nil
			case "/v1.30/info":
				cancel()
				return nil, ctx.Err()
			}
			t.Errorf("unexpected request to %s %s", r.Method, r.URL)
			return nil, fmt.Errorf("unexpected request")
		}
		info, err = client.Info(ctx)
		assert.Check(t, is.DeepEqual(types.Info{}, info))
		assert.Check(t, is.ErrorIs(err, context.Canceled))

		// Get info without any context cancelation shenanigans.
		// No version negotiation should take place.
		expectedInfo := types.Info{Name: "fake-info"}
		infoJSON, err := json.Marshal(&expectedInfo)
		assert.NilError(t, err)
		handler = func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/v1.30/info" {
				header := make(http.Header)
				header.Set("Content-Type", "application/json")
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     header,
					Body:       io.NopCloser(bytes.NewReader(infoJSON)),
				}, nil
			}
			t.Errorf("unexpected request to %s %s", r.Method, r.URL)
			return nil, fmt.Errorf("unexpected request")
		}
		info, err = client.Info(context.Background())
		assert.Check(t, err)
		assert.Check(t, is.DeepEqual(info, expectedInfo))
	})
}
