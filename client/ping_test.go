package client // import "github.com/docker/docker/client"

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestPingFail tests that when a server sends a non-successful response that we
// can still grab API details, when set.
// Some of this is just exercising the code paths to make sure there are no
// panics.
func TestPingFail(t *testing.T) {
	var withHeader bool
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusInternalServerError}
			if withHeader {
				resp.Header = http.Header{}
				resp.Header.Set("Api-Version", "awesome")
				resp.Header.Set("Docker-Experimental", "true")
				resp.Header.Set("Swarm", "inactive")
			}
			resp.Body = io.NopCloser(strings.NewReader("some error with the server"))
			return resp, nil
		}),
	}

	ping, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, "some error with the server"))
	assert.Check(t, is.Equal(false, ping.Experimental))
	assert.Check(t, is.Equal("", ping.APIVersion))
	var si *swarm.Status
	assert.Check(t, is.Equal(si, ping.SwarmStatus))

	withHeader = true
	ping2, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, "some error with the server"))
	assert.Check(t, is.Equal(true, ping2.Experimental))
	assert.Check(t, is.Equal("awesome", ping2.APIVersion))
	assert.Check(t, is.Equal(swarm.Status{NodeState: "inactive"}, *ping2.SwarmStatus))
}

// TestPingWithError tests the case where there is a protocol error in the ping.
// This test is mostly just testing that there are no panics in this code path.
func TestPingWithError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("some connection error")
		}),
	}

	ping, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, "some connection error"))
	assert.Check(t, is.Equal(false, ping.Experimental))
	assert.Check(t, is.Equal("", ping.APIVersion))
	var si *swarm.Status
	assert.Check(t, is.Equal(si, ping.SwarmStatus))
}

// TestPingSuccess tests that we are able to get the expected API headers/ping
// details on success.
func TestPingSuccess(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusOK}
			resp.Header = http.Header{}
			resp.Header.Set("Api-Version", "awesome")
			resp.Header.Set("Docker-Experimental", "true")
			resp.Header.Set("Swarm", "active/manager")
			resp.Body = io.NopCloser(strings.NewReader("OK"))
			return resp, nil
		}),
	}
	ping, err := client.Ping(context.Background())
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, ping.Experimental))
	assert.Check(t, is.Equal("awesome", ping.APIVersion))
	assert.Check(t, is.Equal(swarm.Status{NodeState: "active", ControlAvailable: true}, *ping.SwarmStatus))
}

func TestPingEngineCapabilities(t *testing.T) {
	testCases := []struct {
		doc           string
		body          string
		expected      *system.Capabilities
		expectedError string
	}{
		{
			doc:      "empty",
			expected: nil,
		},
		{
			doc:      "older daemons",
			body:     "OK",
			expected: nil,
		},
		{
			doc:  "valid single",
			body: `{"capabilities": {"_v":1, "data": { "registry-client-auth": true } } }`,
			expected: &system.Capabilities{
				Version: 1,
				Data: map[string]any{
					"registry-client-auth": true,
				},
			},
		},
		{
			doc:  "known and unknown keys",
			body: `{"capabilities":  {"_v":1, "data": { "meow": false, "registry-client-auth": true} } }`,
			expected: &system.Capabilities{
				Version: 1,
				Data: map[string]any{
					"registry-client-auth": true,
					"meow":                 false,
				},
			},
		},
		{
			doc:           "invalid body",
			body:          "bork",
			expectedError: "failed to parse ping body: expected capabilities, found 'bork'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					capabilitiesQuery := req.URL.Query()["capabilities"][0]
					assert.Equal(t, capabilitiesQuery, "1")

					resp := &http.Response{StatusCode: http.StatusOK}
					resp.Header = http.Header{}
					resp.Header.Set("API-Version", "awesome")
					resp.Body = io.NopCloser(strings.NewReader(tc.body))

					return resp, nil
				}),
			}

			ping, err := client.PingWithOptions(context.Background(), types.PingOptions{Capabilities: true})
			if tc.expectedError == "" {
				assert.NilError(t, err)
				assert.Check(t, is.Equal("awesome", ping.APIVersion))
				assert.DeepEqual(t, tc.expected, ping.Capabilities)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}

// TestPingHeadFallback tests that the client falls back to GET if HEAD fails.
func TestPingHeadFallback(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{
			status:   http.StatusOK,
			expected: http.MethodHead,
		},
		{
			status:   http.StatusInternalServerError,
			expected: http.MethodHead,
		},
		{
			status:   http.StatusNotFound,
			expected: "HEAD, GET",
		},
		{
			status:   http.StatusMethodNotAllowed,
			expected: "HEAD, GET",
		},
	}

	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var reqs []string
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					reqs = append(reqs, req.Method)
					resp := &http.Response{StatusCode: http.StatusOK}
					if req.Method == http.MethodHead {
						resp.StatusCode = tc.status
					}
					resp.Header = http.Header{}
					resp.Header.Add("Api-Version", strings.Join(reqs, ", "))
					return resp, nil
				}),
			}
			ping, _ := client.Ping(context.Background())
			assert.Check(t, is.Equal(ping.APIVersion, tc.expected))
		})
	}
}
