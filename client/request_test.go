package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/common"
	"github.com/moby/moby/api/types/container"
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
			host:            "unix:///var/run/docker.sock",
			expectedHost:    DummyHost,
			expectedURLHost: "/var/run/docker.sock",
		},
		{
			host:            "npipe:////./pipe/docker_engine",
			expectedHost:    DummyHost,
			expectedURLHost: "//./pipe/docker_engine",
		},
		{
			host:            "tcp://0.0.0.0:4243",
			expectedHost:    "",
			expectedURLHost: "0.0.0.0:4243",
		},
		{
			host:            "tcp://localhost:4243",
			expectedHost:    "",
			expectedURLHost: "localhost:4243",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.host, func(t *testing.T) {
			hostURL, err := ParseHostURL(tc.host)
			assert.NilError(t, err)

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
			assert.NilError(t, err)
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
	_, err := client.ContainerList(context.Background(), container.ListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

// TestResponseErrors tests handling of error responses returned by the API.
// It includes test-cases for malformed and invalid error-responses, as well
// as plain text errors for backwards compatibility with API versions <1.24.
func TestResponseErrors(t *testing.T) {
	errorResponse, err := json.Marshal(&common.ErrorResponse{
		Message: "Some error occurred",
	})
	assert.NilError(t, err)

	tests := []struct {
		doc         string
		apiVersion  string
		contentType string
		response    string
		expected    string
	}{
		{
			// Valid [common.ErrorResponse] error, but not using a fixture, to validate current implementation..
			doc:         "JSON error (non-fixture)",
			contentType: "application/json",
			response:    string(errorResponse),
			expected:    `Error response from daemon: Some error occurred`,
		},
		{
			// Valid [common.ErrorResponse] error.
			doc:         "JSON error",
			contentType: "application/json",
			response:    `{"message":"Some error occurred"}`,
			expected:    `Error response from daemon: Some error occurred`,
		},
		{
			// Valid [common.ErrorResponse] error with additional fields.
			doc:         "JSON error with extra fields",
			contentType: "application/json",
			response:    `{"message":"Some error occurred", "other_field": "some other field that's not part of common.ErrorResponse"}`,
			expected:    `Error response from daemon: Some error occurred`,
		},
		{
			// API versions before 1.24 did not support JSON errors. Technically,
			// we no longer downgrade to older API versions, but we make an
			// exception for errors so that older clients would print a more
			// readable error.
			doc:         "JSON error on old API",
			apiVersion:  "1.23",
			contentType: "text/plain; charset=utf-8",
			response:    `client version 1.10 is too old. Minimum supported API version is 1.24, please upgrade your client to a newer version`,
			expected:    `Error response from daemon: client version 1.10 is too old. Minimum supported API version is 1.24, please upgrade your client to a newer version`,
		},
		{
			doc:         "plain-text error",
			contentType: "text/plain",
			response:    `Some error occurred`,
			expected:    `Error response from daemon: Some error occurred`,
		},
		{
			// TODO(thaJeztah): consider returning (partial) raw response for these
			doc:         "malformed JSON",
			contentType: "application/json",
			response:    `{"message":"Some error occurred`,
			expected:    `error reading JSON: unexpected end of JSON input`,
		},
		{
			// Server response that's valid JSON, but not the expected [common.ErrorResponse] scheme
			doc:         "incorrect JSON scheme",
			contentType: "application/json",
			response:    `{"error":"Some error occurred"}`,
			expected:    `Error response from daemon: API returned a 400 (Bad Request) but provided no error-message`,
		},
		{
			// TODO(thaJeztah): improve handling of such errors; we can return the generic "502 Bad Gateway" instead
			doc:         "html error",
			contentType: "text/html",
			response: `<!doctype html>
<html lang="en">
<head>
  <title>502 Bad Gateway</title>
</head>
<body>
  <h1>Bad Gateway</h1>
  <p>The server was unable to complete your request. Please try again later.</p>
  <p>If this problem persists, please <a href="https://example.com/support">contact support</a>.</p>
</body>
</html>`,
			expected: `Error response from daemon: <!doctype html>
<html lang="en">
<head>
  <title>502 Bad Gateway</title>
</head>
<body>
  <h1>Bad Gateway</h1>
  <p>The server was unable to complete your request. Please try again later.</p>
  <p>If this problem persists, please <a href="https://example.com/support">contact support</a>.</p>
</body>
</html>`,
		},
		{
			// TODO(thaJeztah): improve handling of these errors (JSON: invalid character '<' looking for beginning of value)
			doc:         "html error masquerading as JSON",
			contentType: "application/json",
			response: `<!doctype html>
<html lang="en">
<head>
  <title>502 Bad Gateway</title>
</head>
<body>
  <h1>Bad Gateway</h1>
  <p>The server was unable to complete your request. Please try again later.</p>
  <p>If this problem persists, please <a href="https://example.com/support">contact support</a>.</p>
</body>
</html>`,
			expected: `error reading JSON: invalid character '<' looking for beginning of value`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				version: tc.apiVersion,
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Header:     http.Header{"Content-Type": []string{tc.contentType}},
						Body:       io.NopCloser(bytes.NewReader([]byte(tc.response))),
					}, nil
				}),
			}
			_, err := client.Ping(context.Background())
			assert.Check(t, is.Error(err, tc.expected))
			assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
		})
	}
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
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
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
	assert.Check(t, is.ErrorIs(err, context.Canceled))
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
	assert.Check(t, is.ErrorIs(err, context.DeadlineExceeded))
}

func TestPrepareJSONRequest(t *testing.T) {
	tests := []struct {
		doc        string
		body       any
		headers    http.Header
		expBody    string
		expNilBody bool
		expHeaders http.Header
	}{
		{
			doc:        "nil body",
			body:       nil,
			headers:    http.Header{"Something": []string{"something"}},
			expNilBody: true,
			expHeaders: http.Header{
				// currently, no content-type is set on empty requests.
				"Something": []string{"something"},
			},
		},
		{
			doc:        "nil interface body",
			body:       (*struct{})(nil),
			headers:    http.Header{"Something": []string{"something"}},
			expNilBody: true,
			expHeaders: http.Header{
				// currently, no content-type is set on empty requests.
				"Something": []string{"something"},
			},
		},
		{
			doc:     "empty struct body",
			body:    &struct{}{},
			headers: http.Header{"Something": []string{"something"}},
			expBody: `{}`,
			expHeaders: http.Header{
				"Content-Type": []string{"application/json"},
				"Something":    []string{"something"},
			},
		},
		{
			doc:     "json raw message",
			body:    json.RawMessage("{}"),
			expBody: `{}`,
			expHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		{
			doc:     "empty body",
			body:    http.NoBody,
			expBody: `{}`,
			expHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			req, hdr, err := prepareJSONRequest(tc.body, tc.headers)
			assert.NilError(t, err)

			var body string
			if tc.expNilBody {
				assert.Check(t, is.Nil(req))
			} else {
				assert.Assert(t, req != nil)

				resp, err := io.ReadAll(req)
				assert.NilError(t, err)
				body = strings.TrimSpace(string(resp))
			}

			assert.Check(t, is.Equal(body, tc.expBody))
			assert.Check(t, is.DeepEqual(hdr, tc.expHeaders))
			assert.Check(t, is.Equal(tc.headers.Get("Content-Type"), ""), "Should not have mutated original headers")
		})
	}
}
