package client // import "github.com/docker/docker/client"

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

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
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusInternalServerError}
			if withHeader {
				resp.Header = http.Header{}
				resp.Header.Set("API-Version", "awesome")
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
			resp.Header.Set("API-Version", "awesome")
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

func TestPingParseEngineFeatures(t *testing.T) {
	tooManyFeaturesHeader := ""
	for i := 0; i < 101; i++ {
		tooManyFeaturesHeader += strconv.Itoa(i) + "=true,"
	}
	tooManyFeaturesHeader = tooManyFeaturesHeader[:len(tooManyFeaturesHeader)-1]
	testCases := []struct {
		doc           string
		in            string
		expected      map[string]bool
		expectedError string
	}{
		{
			doc:      "empty",
			in:       "",
			expected: map[string]bool{},
		},
		{
			doc: "valid single",
			in:  "foo=true",
			expected: map[string]bool{
				"foo": true,
			},
		},
		{
			doc: "valid multiple",
			in:  "bork=false,meow-snapshotter=true",
			expected: map[string]bool{
				"bork":             false,
				"meow-snapshotter": true,
			},
		},
		{
			doc:           "invalid: missing '='",
			in:            "bork",
			expectedError: "failed to parse Engine-Features header: feature 'bork' is missing '='",
		},
		{
			doc:           "invalid: too many '='",
			in:            "bork=meow=false",
			expectedError: "failed to parse Engine-Features header: feature 'bork=meow=false' has too many '='",
		},
		{
			doc:           "multiple invalid",
			in:            "bork=meow=false,foo",
			expectedError: "failed to parse Engine-Features header: feature 'bork=meow=false' has too many '='",
		},
		{
			doc:           "valid + invalid features",
			in:            "foo=true,bar",
			expectedError: "failed to parse Engine-Features header: feature 'bar' is missing '='",
		},
		{
			doc:           "duplicate key",
			in:            "foo=false,foo=true",
			expectedError: "failed to parse Engine-Features header: duplicate feature 'foo'",
		},
		{
			doc:           "too many features",
			in:            tooManyFeaturesHeader,
			expectedError: "failed to parse Engine-Features header: too many features: expected max 100, found 101",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					resp := &http.Response{StatusCode: http.StatusOK}
					resp.Header = http.Header{}
					resp.Header.Set("API-Version", "awesome")
					resp.Header.Set("Engine-Features", tc.in)
					resp.Body = io.NopCloser(strings.NewReader("OK"))
					return resp, nil
				}),
			}
			ping, err := client.Ping(context.Background())
			if tc.expectedError == "" {
				assert.NilError(t, err)
				assert.Check(t, is.Equal("awesome", ping.APIVersion))
				assert.DeepEqual(t, tc.expected, ping.EngineFeatures)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}

func TestParseEngineFeaturesHeaderMultipleHeaders(t *testing.T) {
	tooManyFeaturesHeader := ""
	for i := 0; i < 51; i++ {
		tooManyFeaturesHeader += strconv.Itoa(i) + "=true,"
	}
	tooManyFeaturesHeader = tooManyFeaturesHeader[:len(tooManyFeaturesHeader)-1]
	tooManyFeaturesHeader2 := ""
	for i := 51; i < 102; i++ {
		tooManyFeaturesHeader2 += strconv.Itoa(i) + "=true,"
	}
	tooManyFeaturesHeader2 = tooManyFeaturesHeader2[:len(tooManyFeaturesHeader2)-1]
	testCases := []struct {
		doc           string
		in            string
		in2           string
		expected      map[string]bool
		expectedError string
	}{
		{
			doc:      "empty",
			in:       "",
			in2:      "",
			expected: map[string]bool{},
		},
		{
			doc: "single + empty",
			in:  "foo=true",
			in2: "",
			expected: map[string]bool{
				"foo": true,
			},
		},
		{
			doc: "empty + single",
			in:  "",
			in2: "bork=false",
			expected: map[string]bool{
				"bork": false,
			},
		},
		{
			doc:           "duplicate features in separate headers",
			in:            "foo=true",
			in2:           "foo=false",
			expectedError: "failed to parse Engine-Features header: duplicate feature 'foo'",
		},
		{
			doc:           "too many features",
			in:            tooManyFeaturesHeader,
			in2:           tooManyFeaturesHeader2,
			expectedError: "failed to parse Engine-Features header: too many features: expected max 100, found 102",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					resp := &http.Response{StatusCode: http.StatusOK}
					resp.Header = http.Header{}
					resp.Header.Set("API-Version", "awesome")
					resp.Header.Set("Engine-Features", tc.in)
					resp.Header.Add("Engine-Features", tc.in2)
					resp.Body = io.NopCloser(strings.NewReader("OK"))
					return resp, nil
				}),
			}
			ping, err := client.Ping(context.Background())
			if tc.expectedError == "" {
				assert.NilError(t, err)
				assert.Check(t, is.Equal("awesome", ping.APIVersion))
				assert.DeepEqual(t, tc.expected, ping.EngineFeatures)
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
					resp.Header.Add("API-Version", strings.Join(reqs, ", "))
					return resp, nil
				}),
			}
			ping, _ := client.Ping(context.Background())
			assert.Check(t, is.Equal(ping.APIVersion, tc.expected))
		})
	}
}
