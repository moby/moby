package client

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestPingFail tests that when a server sends a non-successful response that we
// can still grab API details, when set.
// Some of this is just exercising the code paths to make sure there are no
// panics.
func TestPingFail(t *testing.T) {
	var withHeader bool
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		var hdr http.Header
		if withHeader {
			hdr = http.Header{}
			hdr.Set("Api-Version", "awesome")
			hdr.Set("Docker-Experimental", "true")
			hdr.Set("Swarm", "inactive")
		}
		return mockResponse(http.StatusInternalServerError, hdr, "some error with the server")(req)
	}))
	assert.NilError(t, err)

	ping, err := client.Ping(t.Context(), PingOptions{})
	assert.Check(t, is.ErrorContains(err, "some error with the server"))
	assert.Check(t, is.Equal(false, ping.Experimental))
	assert.Check(t, is.Equal("", ping.APIVersion))
	var si *SwarmStatus
	assert.Check(t, is.Equal(si, ping.SwarmStatus))

	withHeader = true
	ping2, err := client.Ping(t.Context(), PingOptions{})
	assert.Check(t, is.ErrorContains(err, "some error with the server"))
	assert.Check(t, is.Equal(true, ping2.Experimental))
	assert.Check(t, is.Equal("awesome", ping2.APIVersion))
	assert.Check(t, is.Equal(SwarmStatus{NodeState: "inactive"}, *ping2.SwarmStatus))
}

// TestPingWithError tests the case where there is a protocol error in the ping.
// This test is mostly just testing that there are no panics in this code path.
func TestPingWithError(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("some connection error")
	}))
	assert.NilError(t, err)

	ping, err := client.Ping(t.Context(), PingOptions{})
	assert.Check(t, is.ErrorContains(err, "some connection error"))
	assert.Check(t, is.Equal(false, ping.Experimental))
	assert.Check(t, is.Equal("", ping.APIVersion))
	var si *SwarmStatus
	assert.Check(t, is.Equal(si, ping.SwarmStatus))
}

// TestPingSuccess tests that we are able to get the expected API headers/ping
// details on success.
func TestPingSuccess(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		hdr := http.Header{}
		hdr.Set("Api-Version", "awesome")
		hdr.Set("Docker-Experimental", "true")
		hdr.Set("Swarm", "active/manager")
		return mockResponse(http.StatusOK, hdr, "OK")(req)
	}))
	assert.NilError(t, err)
	ping, err := client.Ping(t.Context(), PingOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, ping.Experimental))
	assert.Check(t, is.Equal("awesome", ping.APIVersion))
	assert.Check(t, is.Equal(MaxAPIVersion, client.version))
	assert.Check(t, is.Equal(SwarmStatus{NodeState: "active", ControlAvailable: true}, *ping.SwarmStatus))
}

// TestPingHeadFallback tests that the client falls back to GET if HEAD fails.
func TestPingHeadFallback(t *testing.T) {
	const expectedPath = "/_ping"
	expMethods := []string{http.MethodHead, http.MethodGet}

	tests := []struct {
		status   int
		expected []string
	}{
		{
			status:   http.StatusOK,
			expected: []string{http.MethodHead},
		},
		{
			status:   http.StatusInternalServerError,
			expected: []string{http.MethodHead, http.MethodGet},
		},
		{
			status:   http.StatusNotFound,
			expected: []string{http.MethodHead, http.MethodGet},
		},
		{
			status:   http.StatusMethodNotAllowed,
			expected: []string{http.MethodHead, http.MethodGet},
		},
	}

	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var reqs []string
			client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedPath) {
					return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedPath, req.URL.Path)
				}
				if !slices.Contains(expMethods, req.Method) {
					return nil, fmt.Errorf("expected one of '%v', got '%s'", expMethods, req.Method)
				}
				reqs = append(reqs, req.Method)
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}
				if req.Method == http.MethodHead {
					resp.StatusCode = tc.status
				}
				resp.Header.Add("Api-Version", "1.2.3")
				return resp, nil
			}))
			assert.NilError(t, err)
			ping, _ := client.Ping(t.Context(), PingOptions{})
			assert.Check(t, is.Equal(ping.APIVersion, "1.2.3"))
			assert.Check(t, is.DeepEqual(reqs, tc.expected))
		})
	}
}
