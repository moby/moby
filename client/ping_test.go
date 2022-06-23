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
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestPingFail tests that when a server sends a non-successful response that we
// can still grab API details, when set.
// Some of this is just exercising the code paths to make sure there are no
// panics.
func TestPingFail(t *testing.T) {
	var withHeader bool
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusInternalServerError}
			if withHeader {
				resp.Header = http.Header{}
				resp.Header.Set("API-Version", "awesome")
				resp.Header.Set("Docker-Experimental", "true")
				resp.Header.Set("Swarm", "inactive")
			}
			resp.Body = io.NopCloser(strings.NewReader("some error with the server"))
			return resp, nil
		})),
	)
	assert.NilError(t, err)

	var want types.Ping
	ping, err := client.Ping(context.Background())
	assert.ErrorContains(t, err, "some error with the server")
	assert.Check(t, is.DeepEqual(ping, want))

	withHeader = true
	ping2, err := client.Ping(context.Background())
	assert.ErrorContains(t, err, "some error with the server")
	want = types.Ping{
		Experimental: true,
		APIVersion:   "awesome",
		SwarmStatus:  &swarm.Status{NodeState: "inactive"},
	}
	assert.Check(t, is.DeepEqual(ping2, want))
}

// TestPingWithError tests the case where there is a protocol error in the ping.
// This test is mostly just testing that there are no panics in this code path.
func TestPingWithError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusInternalServerError}
			resp.Header = http.Header{}
			resp.Header.Set("API-Version", "awesome")
			resp.Header.Set("Docker-Experimental", "true")
			resp.Header.Set("Swarm", "active/manager")
			resp.Body = io.NopCloser(strings.NewReader("some error with the server"))
			return resp, errors.New("some error")
		})),
	)
	assert.NilError(t, err)

	ping, err := client.Ping(context.Background())
	assert.ErrorContains(t, err, "some error")
	assert.Check(t, is.Equal(false, ping.Experimental))
	assert.Check(t, is.Equal("", ping.APIVersion))
	var si *swarm.Status
	assert.Check(t, is.Equal(si, ping.SwarmStatus))
}

// TestPingSuccess tests that we are able to get the expected API headers/ping
// details on success.
func TestPingSuccess(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusOK}
			resp.Header = http.Header{}
			resp.Header.Set("API-Version", "awesome")
			resp.Header.Set("Docker-Experimental", "true")
			resp.Header.Set("Swarm", "active/manager")
			resp.Body = io.NopCloser(strings.NewReader("OK"))
			return resp, nil
		})),
	)
	assert.NilError(t, err)
	ping, err := client.Ping(context.Background())
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, ping.Experimental))
	assert.Check(t, is.Equal("awesome", ping.APIVersion))
	assert.Check(t, is.Equal(swarm.Status{NodeState: "active", ControlAvailable: true}, *ping.SwarmStatus))
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
		tc := tc
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var reqs []string
			client, err := NewClientWithOpts(
				WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
					reqs = append(reqs, req.Method)
					resp := &http.Response{StatusCode: http.StatusOK}
					if req.Method == http.MethodHead {
						resp.StatusCode = tc.status
					}
					resp.Header = http.Header{}
					resp.Header.Add("API-Version", strings.Join(reqs, ", "))
					return resp, nil
				})),
			)
			assert.NilError(t, err)
			ping, _ := client.Ping(context.Background())
			assert.Check(t, is.Equal(ping.APIVersion, tc.expected))
		})
	}
}
