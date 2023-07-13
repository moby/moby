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
	const testEndpoint = "/test"
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

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.host, func(t *testing.T) {
			hostURL, err := ParseHostURL(tc.host)
			assert.Check(t, err)

			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					if !strings.HasPrefix(req.URL.Path, testEndpoint) {
						return nil, fmt.Errorf("expected URL %q, got %q", testEndpoint, req.URL)
					}
					if req.Host != tc.expectedHost {
						return nil, fmt.Errorf("wxpected host %q, got %q", tc.expectedHost, req.Host)
					}
					if req.URL.Host != tc.expectedURLHost {
						return nil, fmt.Errorf("expected URL host %q, got %q", tc.expectedURLHost, req.URL.Host)
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

			_, err = client.sendRequest(context.Background(), http.MethodGet, testEndpoint, nil, nil, nil)
			assert.Check(t, err)
		})
	}
}

// TestPlainTextError tests the server returning an error in plain text for
// backwards compatibility with API versions <1.24. All other tests use
// errors returned as JSON
func TestPlainTextError(t *testing.T) {
	client := &Client{
		client: newMockClient(plainTextErrorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerList(context.Background(), types.ContainerListOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestInfiniteError(t *testing.T) {
	infinitR := rand.New(rand.NewSource(42))
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{},
				Body:       io.NopCloser(infinitR),
			}
			return resp, nil
		}),
	}

	_, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, "request returned Internal Server Error"))
}

func TestCanceledContext(t *testing.T) {
	const testEndpoint = "/test"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			assert.Check(t, is.ErrorType(req.Context().Err(), context.Canceled))
			return nil, context.Canceled
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.sendRequest(ctx, http.MethodGet, testEndpoint, nil, nil, nil)
	assert.Check(t, is.ErrorType(err, errdefs.IsCancelled))
	assert.Check(t, errors.Is(err, context.Canceled))
}

func TestDeadlineExceededContext(t *testing.T) {
	const testEndpoint = "/test"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			assert.Check(t, is.ErrorType(req.Context().Err(), context.DeadlineExceeded))
			return nil, context.DeadlineExceeded
		}),
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()

	<-ctx.Done()

	_, err := client.sendRequest(ctx, http.MethodGet, testEndpoint, nil, nil, nil)
	assert.Check(t, is.ErrorType(err, errdefs.IsDeadline))
	assert.Check(t, errors.Is(err, context.DeadlineExceeded))
}
