package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestNewClientWithOpsFromEnv(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")

	testcases := []struct {
		doc             string
		envs            map[string]string
		expectedError   string
		expectedVersion string
	}{
		{
			doc:             "default api version",
			envs:            map[string]string{},
			expectedVersion: MaxAPIVersion,
		},
		{
			doc: "invalid cert path",
			envs: map[string]string{
				"DOCKER_CERT_PATH": "invalid/path",
			},
			expectedError: "could not load X509 key pair: open invalid/path/cert.pem: no such file or directory",
		},
		{
			doc: "default api version with cert path",
			envs: map[string]string{
				"DOCKER_CERT_PATH": "testdata/",
			},
			expectedVersion: MaxAPIVersion,
		},
		{
			doc: "default api version with cert path and tls verify",
			envs: map[string]string{
				"DOCKER_CERT_PATH":  "testdata/",
				"DOCKER_TLS_VERIFY": "1",
			},
			expectedVersion: MaxAPIVersion,
		},
		{
			doc: "default api version with cert path and host",
			envs: map[string]string{
				"DOCKER_CERT_PATH": "testdata/",
				"DOCKER_HOST":      "https://notaunixsocket",
			},
			expectedVersion: MaxAPIVersion,
		},
		{
			doc: "invalid docker host",
			envs: map[string]string{
				"DOCKER_HOST": "host",
			},
			expectedError: "unable to parse docker host `host`",
		},
		{
			doc: "invalid docker host, with good format",
			envs: map[string]string{
				"DOCKER_HOST": "invalid://url",
			},
			expectedVersion: MaxAPIVersion,
		},
		{
			doc: "override api version",
			envs: map[string]string{
				"DOCKER_API_VERSION": "1.50",
			},
			expectedVersion: "1.50",
		},
		{
			doc: "override with unsupported api version",
			envs: map[string]string{
				"DOCKER_API_VERSION": "1.0",
			},
			expectedVersion: "1.0",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.doc, func(t *testing.T) {
			for key, value := range tc.envs {
				t.Setenv(key, value)
			}
			client, err := New(FromEnv)
			if tc.expectedError != "" {
				assert.Check(t, is.Error(err, tc.expectedError))
			} else {
				assert.NilError(t, err)
				assert.Check(t, is.Equal(client.ClientVersion(), tc.expectedVersion))
			}

			if tc.envs["DOCKER_TLS_VERIFY"] != "" {
				// pedantic checking that this is handled correctly
				tlsConfig := client.tlsConfig()
				assert.Assert(t, tlsConfig != nil)
				assert.Check(t, is.Equal(tlsConfig.InsecureSkipVerify, false))
			}
		})
	}
}

func TestGetAPIPath(t *testing.T) {
	tests := []struct {
		version  string
		path     string
		query    url.Values
		expected string
	}{
		{
			path:     "/containers/json",
			expected: "/v" + MaxAPIVersion + "/containers/json",
		},
		{
			path:     "/containers/json",
			query:    url.Values{},
			expected: "/v" + MaxAPIVersion + "/containers/json",
		},
		{
			path:     "/containers/json",
			query:    url.Values{"s": []string{"c"}},
			expected: "/v" + MaxAPIVersion + "/containers/json?s=c",
		},
		{
			version:  "1.50",
			path:     "/containers/json",
			expected: "/v1.50/containers/json",
		},
		{
			version:  "1.50",
			path:     "/containers/json",
			query:    url.Values{},
			expected: "/v1.50/containers/json",
		},
		{
			version:  "1.50",
			path:     "/containers/json",
			query:    url.Values{"s": []string{"c"}},
			expected: "/v1.50/containers/json?s=c",
		},
		{
			version:  "v1.50",
			path:     "/containers/json",
			expected: "/v1.50/containers/json",
		},
		{
			version:  "v1.50",
			path:     "/containers/json",
			query:    url.Values{},
			expected: "/v1.50/containers/json",
		},
		{
			version:  "v1.50",
			path:     "/containers/json",
			query:    url.Values{"s": []string{"c"}},
			expected: "/v1.50/containers/json?s=c",
		},
		{
			version:  "v1.50",
			path:     "/networks/kiwl$%^",
			expected: "/v1.50/networks/kiwl$%25%5E",
		},
	}

	ctx := context.TODO()
	for _, tc := range tests {
		client, err := New(
			WithAPIVersion(tc.version),
			WithHost("tcp://localhost:2375"),
		)
		assert.NilError(t, err)
		actual := client.getAPIPath(ctx, tc.path, tc.query)
		assert.Check(t, is.Equal(actual, tc.expected))
	}
}

func TestParseHostURL(t *testing.T) {
	testcases := []struct {
		host        string
		expected    *url.URL
		expectedErr string
	}{
		{
			host:        "",
			expectedErr: "unable to parse docker host",
		},
		{
			host:        "foobar",
			expectedErr: "unable to parse docker host",
		},
		{
			host:     "foo://bar",
			expected: &url.URL{Scheme: "foo", Host: "bar"},
		},
		{
			host:     "tcp://localhost:2476",
			expected: &url.URL{Scheme: "tcp", Host: "localhost:2476"},
		},
		{
			host:     "tcp://localhost:2476/path",
			expected: &url.URL{Scheme: "tcp", Host: "localhost:2476", Path: "/path"},
		},
		{
			host:     "unix:///var/run/docker.sock",
			expected: &url.URL{Scheme: "unix", Host: "/var/run/docker.sock"},
		},
		{
			host:     "npipe:////./pipe/docker_engine",
			expected: &url.URL{Scheme: "npipe", Host: "//./pipe/docker_engine"},
		},
	}

	for _, testcase := range testcases {
		actual, err := ParseHostURL(testcase.host)
		if testcase.expectedErr != "" {
			assert.Check(t, is.ErrorContains(err, testcase.expectedErr))
		}
		assert.Check(t, is.DeepEqual(actual, testcase.expected))
	}
}

func TestNewClientWithOpsFromEnvSetsDefaultVersion(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_API_VERSION", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	client, err := New(FromEnv)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(client.ClientVersion(), MaxAPIVersion))

	const expected = "1.50"
	t.Setenv("DOCKER_API_VERSION", expected)
	client, err = New(FromEnv)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(client.ClientVersion(), expected))
}

// TestNegotiateAPIVersionEmpty asserts that client.Client version negotiation
// downgrades to the correct API version if the API's ping response does not
// return an API version.
func TestNegotiateAPIVersionEmpty(t *testing.T) {
	t.Setenv("DOCKER_API_VERSION", "")

	// if no version from server, expect the earliest
	// version before APIVersion was implemented
	const expected = MinAPIVersion

	client, err := New(FromEnv,
		WithBaseMockClient(mockPingResponse(http.StatusOK, PingResult{APIVersion: expected})),
	)
	assert.NilError(t, err)

	// set our version to something new.
	// we're not using [WithVersion] here, as that marks the version
	// as manually overridden.
	client.version = "1.51"

	// test downgrade
	ping, err := client.Ping(t.Context(), PingOptions{
		NegotiateAPIVersion: true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(ping.APIVersion, expected))
	assert.Check(t, is.Equal(client.ClientVersion(), expected))
}

// TestNegotiateAPIVersion asserts that client.Client can
// negotiate a compatible APIVersion with the server
func TestNegotiateAPIVersion(t *testing.T) {
	tests := []struct {
		doc             string
		clientVersion   string
		pingVersion     string
		expectedVersion string
		expectedErr     string
	}{
		{
			// client should downgrade to the version reported by the daemon.
			doc:             "downgrade from default",
			pingVersion:     "1.50",
			expectedVersion: "1.50",
		},
		{
			// client should not downgrade to the version reported by the
			// daemon if a custom version was set.
			doc:             "no downgrade from custom version",
			clientVersion:   "1.51",
			pingVersion:     "1.50",
			expectedVersion: "1.51",
		},
		{
			// client should not downgrade if the daemon didn't report a version.
			doc:             "downgrade legacy",
			pingVersion:     "",
			expectedVersion: MaxAPIVersion,
		},
		{
			// client should not downgrade to the version reported by the daemon
			// if the version is not supported.
			doc:             "no downgrade old",
			pingVersion:     "1.19",
			expectedVersion: MaxAPIVersion,
			expectedErr:     "API version 1.19 is not supported by this client: the minimum supported API version is " + MinAPIVersion,
		},
		{
			// client should not upgrade to a newer version if a version was set,
			// even if both the daemon and the client support it.
			doc:             "no upgrade",
			clientVersion:   "1.50",
			pingVersion:     "1.51",
			expectedVersion: "1.50",
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			opts := []Opt{
				FromEnv,
				WithBaseMockClient(mockPingResponse(http.StatusOK, PingResult{APIVersion: tc.pingVersion})),
			}

			if tc.clientVersion != "" {
				// Note that this check is redundant, as WithVersion() considers
				// an empty version equivalent to "not setting a version", but
				// doing this just to be explicit we are using the default.
				opts = append(opts, WithAPIVersion(tc.clientVersion))
			}
			client, err := New(opts...)
			assert.NilError(t, err)
			_, err = client.Ping(t.Context(), PingOptions{
				NegotiateAPIVersion: true,
			})
			if tc.expectedErr != "" {
				assert.Check(t, is.ErrorContains(err, tc.expectedErr))
			} else {
				assert.NilError(t, err)
			}
			assert.Check(t, is.Equal(tc.expectedVersion, client.ClientVersion()))
		})
	}
}

// TestNegotiateAPIVersionOverride asserts that we honor the DOCKER_API_VERSION
// environment variable when negotiating versions.
func TestNegotiateAPIVersionOverride(t *testing.T) {
	const expected = "9.99"
	t.Setenv("DOCKER_API_VERSION", expected)

	client, err := New(
		FromEnv,
		WithBaseMockClient(mockPingResponse(http.StatusOK, PingResult{APIVersion: "1.45"})),
	)
	assert.NilError(t, err)

	// test that we honored the env var
	_, err = client.Ping(t.Context(), PingOptions{
		NegotiateAPIVersion: true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(client.ClientVersion(), expected))
}

// TestNegotiateAPIVersionConnectionFailure asserts that we do not modify the
// API version when failing to connect.
func TestNegotiateAPIVersionConnectionFailure(t *testing.T) {
	const expected = "9.99"

	client, err := New(WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)
	client.version = expected
	_, err = client.Ping(t.Context(), PingOptions{
		NegotiateAPIVersion: true,
	})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
	assert.Check(t, is.ErrorContains(err, `failed to connect to the docker API at tcp://no-such-host.invalid`))
	assert.Check(t, is.Equal(client.ClientVersion(), expected))
}

func TestNegotiateAPIVersionAutomatic(t *testing.T) {
	var pingVersion string

	ctx := t.Context()
	client, err := New(
		WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
			return mockPingResponse(http.StatusOK, PingResult{APIVersion: pingVersion})(req)
		}),
	)
	assert.NilError(t, err)

	// Client defaults to use MaxAPIVersion before version-negotiation.
	expected := MaxAPIVersion
	assert.Check(t, is.Equal(client.ClientVersion(), expected))

	// First request should trigger negotiation
	pingVersion = "1.50"
	expected = "1.50"
	_, _ = client.Info(ctx, InfoOptions{})
	assert.Check(t, is.Equal(client.ClientVersion(), expected))

	// Once successfully negotiated, subsequent requests should not re-negotiate
	pingVersion = "1.49"
	expected = "1.50"
	_, _ = client.Info(ctx, InfoOptions{})
	assert.Check(t, is.Equal(client.ClientVersion(), expected))
}

// TestNegotiateAPIVersionWithEmptyVersion asserts that initializing a client
// with an empty version string does still allow API-version negotiation
func TestNegotiateAPIVersionWithEmptyVersion(t *testing.T) {
	client, err := New(
		WithAPIVersion(""),
		WithBaseMockClient(mockPingResponse(http.StatusOK, PingResult{APIVersion: "1.50"})),
	)
	assert.NilError(t, err)

	const expected = "1.50"
	_, err = client.Ping(t.Context(), PingOptions{
		NegotiateAPIVersion: true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(client.ClientVersion(), expected))
}

// TestNegotiateAPIVersionWithFixedVersion asserts that initializing a client
// with a fixed version disables API-version negotiation
func TestNegotiateAPIVersionWithFixedVersion(t *testing.T) {
	const (
		customVersion = "1.50"
		pingVersion   = "1.49"
	)
	client, err := New(
		WithAPIVersion(customVersion),
		WithBaseMockClient(mockPingResponse(http.StatusOK, PingResult{APIVersion: pingVersion})),
	)
	assert.NilError(t, err)

	_, err = client.Ping(t.Context(), PingOptions{
		NegotiateAPIVersion: true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(client.ClientVersion(), customVersion))

	_, err = client.Ping(t.Context(), PingOptions{
		NegotiateAPIVersion: true,
		ForceNegotiate:      true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(client.ClientVersion(), pingVersion))
}

func TestClientRedirect(t *testing.T) {
	client := &http.Client{
		CheckRedirect: CheckRedirect,
		Transport: ensureBody(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "/bla" {
				return mockResponse(http.StatusNotFound, nil, "")(req)
			}
			return mockResponse(http.StatusMovedPermanently, http.Header{"Location": {"/bla"}}, "")(req)
		}),
	}

	tests := []struct {
		httpMethod  string
		expectedErr *url.Error
		statusCode  int
	}{
		{
			httpMethod: http.MethodGet,
			statusCode: http.StatusMovedPermanently,
		},
		{
			httpMethod:  http.MethodPost,
			expectedErr: &url.Error{Op: "Post", URL: "/bla", Err: ErrRedirect},
			statusCode:  http.StatusMovedPermanently,
		},
		{
			httpMethod:  http.MethodPut,
			expectedErr: &url.Error{Op: "Put", URL: "/bla", Err: ErrRedirect},
			statusCode:  http.StatusMovedPermanently,
		},
		{
			httpMethod:  http.MethodDelete,
			expectedErr: &url.Error{Op: "Delete", URL: "/bla", Err: ErrRedirect},
			statusCode:  http.StatusMovedPermanently,
		},
	}

	for _, tc := range tests {
		t.Run(tc.httpMethod, func(t *testing.T) {
			req, err := http.NewRequest(tc.httpMethod, "/redirectme", http.NoBody)
			assert.NilError(t, err)
			resp, err := client.Do(req)
			assert.Check(t, is.Equal(resp.StatusCode, tc.statusCode))
			if tc.expectedErr == nil {
				assert.NilError(t, err)
			} else {
				assert.Check(t, is.ErrorType(err, &url.Error{}))
				var urlError *url.Error
				assert.Check(t, errors.As(err, &urlError), "%T is not *url.Error", err)
				assert.Check(t, is.Equal(*urlError, *tc.expectedErr))
			}
		})
	}
}
