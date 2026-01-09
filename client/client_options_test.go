package client

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestOptionWithHostFromEnv(t *testing.T) {
	c, err := New(WithHostFromEnv())
	assert.NilError(t, err)
	assert.Check(t, c.client != nil)
	assert.Check(t, is.Equal(c.basePath, ""))
	if runtime.GOOS == "windows" {
		assert.Check(t, is.Equal(c.host, "npipe:////./pipe/docker_engine"))
		assert.Check(t, is.Equal(c.proto, "npipe"))
		assert.Check(t, is.Equal(c.addr, "//./pipe/docker_engine"))
	} else {
		assert.Check(t, is.Equal(c.host, "unix:///var/run/docker.sock"))
		assert.Check(t, is.Equal(c.proto, "unix"))
		assert.Check(t, is.Equal(c.addr, "/var/run/docker.sock"))
	}

	t.Setenv("DOCKER_HOST", "tcp://foo.example.com:2376/test/")

	c, err = New(WithHostFromEnv())
	assert.NilError(t, err)
	assert.Check(t, c.client != nil)
	assert.Check(t, is.Equal(c.basePath, "/test/"))
	assert.Check(t, is.Equal(c.host, "tcp://foo.example.com:2376/test/"))
	assert.Check(t, is.Equal(c.proto, "tcp"))
	assert.Check(t, is.Equal(c.addr, "foo.example.com:2376"))
}

func TestOptionWithTimeout(t *testing.T) {
	timeout := 10 * time.Second
	c, err := New(WithTimeout(timeout))
	assert.NilError(t, err)
	assert.Check(t, c.client != nil)
	assert.Check(t, is.Equal(c.client.Timeout, timeout))
}

func TestOptionWithAPIVersion(t *testing.T) {
	tests := []struct {
		doc      string
		version  string
		expected string
		expError string
	}{
		{
			doc:      "empty version",
			version:  "",
			expected: MaxAPIVersion,
		},
		{
			doc:      "custom lower version with whitespace, no v-prefix",
			version:  "   1.50   ",
			expected: "1.50",
		},
		{
			// We currently allow downgrading the client to an unsupported lower version for testing.
			doc:      "downgrade unsupported version, no v-prefix",
			version:  "1.0",
			expected: "1.0",
		},
		{
			doc:      "custom lower version, no v-prefix",
			version:  "1.50",
			expected: "1.50",
		},
		{
			// We currently allow upgrading the client to an unsupported higher version for testing.
			doc:      "upgrade version, no v-prefix",
			version:  "9.99",
			expected: "9.99",
		},
		{
			doc:      "empty version, with v-prefix",
			version:  "v",
			expected: MaxAPIVersion,
		},
		{
			doc:      "whitespace, with v-prefix",
			version:  "   v1.0   ",
			expected: "1.0",
		},
		{
			doc:      "downgrade unsupported version, with v-prefix",
			version:  "v1.0",
			expected: "1.0",
		},
		{
			doc:      "custom lower version with whitespace and v-prefix",
			version:  "   v1.50   ",
			expected: "1.50",
		},
		{
			doc:      "custom lower version, with v-prefix",
			version:  "v1.50",
			expected: "1.50",
		},
		{
			doc:      "upgrade version, with v-prefix",
			version:  "v9.99",
			expected: "9.99",
		},
		{
			doc:      "malformed version",
			version:  "something-weird",
			expError: "invalid API version (something-weird): must be formatted <major>.<minor>",
		},
		{
			doc:      "no minor",
			version:  "1",
			expError: "invalid API version (1): must be formatted <major>.<minor>",
		},
		{
			doc:      "too many digits",
			version:  "1.2.3",
			expError: "invalid API version (1.2.3): invalid minor version: must be formatted <major>.<minor>",
		},
		{
			doc:      "embedded whitespace",
			version:  "v 1.0",
			expError: "invalid API version (v 1.0): invalid major version: must be formatted <major>.<minor>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client, err := New(WithAPIVersion(tc.version))
			if tc.expError != "" {
				assert.Check(t, is.ErrorContains(err, tc.expError))
				assert.Check(t, client == nil)
			} else {
				assert.NilError(t, err)
				assert.Check(t, client != nil)
				assert.Check(t, is.Equal(client.ClientVersion(), tc.expected))
				isNoOp := strings.TrimPrefix(strings.TrimSpace(tc.version), "v") == ""
				assert.Check(t, is.Equal(client.negotiated.Load(), !isNoOp))
			}
		})
	}
}

func TestOptionWithAPIVersionFromEnv(t *testing.T) {
	tests := []struct {
		doc      string
		version  string
		expected string
		expError string
	}{
		{
			doc:      "empty version",
			version:  "",
			expected: MaxAPIVersion,
		},
		{
			doc:      "custom lower version with whitespace, no v-prefix",
			version:  "   1.50   ",
			expected: "1.50",
		},
		{
			// We currently allow downgrading the client to an unsupported lower version for testing.
			doc:      "downgrade unsupported version, no v-prefix",
			version:  "1.0",
			expected: "1.0",
		},
		{
			doc:      "custom lower version, no v-prefix",
			version:  "1.50",
			expected: "1.50",
		},
		{
			// We currently allow upgrading the client to an unsupported higher version for testing.
			doc:      "upgrade version, no v-prefix",
			version:  "9.99",
			expected: "9.99",
		},
		{
			doc:      "empty version, with v-prefix",
			version:  "v",
			expected: MaxAPIVersion,
		},
		{
			doc:      "whitespace, with v-prefix",
			version:  "   v1.0   ",
			expected: "1.0",
		},
		{
			doc:      "downgrade unsupported version, with v-prefix",
			version:  "v1.0",
			expected: "1.0",
		},
		{
			doc:      "custom lower version with whitespace and v-prefix",
			version:  "   v1.50   ",
			expected: "1.50",
		},
		{
			doc:      "custom lower version, with v-prefix",
			version:  "v1.50",
			expected: "1.50",
		},
		{
			doc:      "upgrade version, with v-prefix",
			version:  "v9.99",
			expected: "9.99",
		},
		{
			doc:      "malformed version",
			version:  "something-weird",
			expError: "invalid API version (something-weird): must be formatted <major>.<minor>",
		},
		{
			doc:      "no minor",
			version:  "1",
			expError: "invalid API version (1): must be formatted <major>.<minor>",
		},
		{
			doc:      "too many digits",
			version:  "1.2.3",
			expError: "invalid API version (1.2.3): invalid minor version: must be formatted <major>.<minor>",
		},
		{
			doc:      "embedded whitespace",
			version:  "v 1.0",
			expError: "invalid API version (v 1.0): invalid major version: must be formatted <major>.<minor>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			t.Setenv(EnvOverrideAPIVersion, tc.version)
			client, err := New(WithAPIVersionFromEnv())
			if tc.expError != "" {
				assert.Check(t, is.ErrorContains(err, tc.expError))
				assert.Check(t, client == nil)
			} else {
				assert.NilError(t, err)
				assert.Check(t, client != nil)
				assert.Check(t, is.Equal(client.ClientVersion(), tc.expected))
				isNoOp := strings.TrimPrefix(strings.TrimSpace(tc.version), "v") == ""
				assert.Check(t, is.Equal(client.negotiated.Load(), !isNoOp))
			}
		})
	}
}

// TestOptionOverridePriority validates that overriding the API version through
// [WithAPIVersionFromEnv] takes precedence over other manual options, regardless
// the order in which they're passed.
func TestOptionOverridePriority(t *testing.T) {
	t.Run("no env-var set", func(t *testing.T) {
		client, err := New(WithAPIVersionFromEnv(), WithAPIVersion("1.50"))
		assert.NilError(t, err)
		assert.Check(t, is.Equal(client.ClientVersion(), "1.50"))
		assert.Check(t, is.Equal(client.negotiated.Load(), true))
	})

	const expected = "1.51"
	t.Setenv(EnvOverrideAPIVersion, expected)

	t.Run("WithAPIVersionFromEnv first", func(t *testing.T) {
		client, err := New(WithAPIVersionFromEnv(), WithAPIVersion("1.50"))
		assert.NilError(t, err)
		assert.Check(t, is.Equal(client.ClientVersion(), expected))
		assert.Check(t, is.Equal(client.negotiated.Load(), true))
	})

	t.Run("WithAPIVersionFromEnv last", func(t *testing.T) {
		client, err := New(WithAPIVersion("1.50"), WithAPIVersionFromEnv())
		assert.NilError(t, err)
		assert.Check(t, is.Equal(client.ClientVersion(), expected))
		assert.Check(t, is.Equal(client.negotiated.Load(), true))
	})

	t.Run("FromEnv first", func(t *testing.T) {
		client, err := New(FromEnv, WithAPIVersion("1.50"))
		assert.NilError(t, err)
		assert.Check(t, is.Equal(client.ClientVersion(), expected))
		assert.Check(t, is.Equal(client.negotiated.Load(), true))
	})

	t.Run("FromEnv last", func(t *testing.T) {
		client, err := New(WithAPIVersion("1.50"), FromEnv)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(client.ClientVersion(), expected))
		assert.Check(t, is.Equal(client.negotiated.Load(), true))
	})
}

func TestWithUserAgent(t *testing.T) {
	const userAgent = "Magic-Client/v1.2.3"
	t.Run("user-agent", func(t *testing.T) {
		c, err := New(
			WithUserAgent(userAgent),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), userAgent))
				return &http.Response{StatusCode: http.StatusOK}, nil
			}),
		)
		assert.NilError(t, err)
		_, err = c.Ping(t.Context(), PingOptions{})
		assert.NilError(t, err)
		assert.NilError(t, c.Close())
	})
	t.Run("user-agent and custom headers", func(t *testing.T) {
		c, err := New(
			WithUserAgent(userAgent),
			WithHTTPHeaders(map[string]string{"User-Agent": "should-be-ignored/1.0.0", "Other-Header": "hello-world"}),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), userAgent))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			}),
		)
		assert.NilError(t, err)
		_, err = c.Ping(t.Context(), PingOptions{})
		assert.NilError(t, err)
		assert.NilError(t, c.Close())
	})
	t.Run("custom headers", func(t *testing.T) {
		c, err := New(
			WithHTTPHeaders(map[string]string{"User-Agent": "from-custom-headers/1.0.0", "Other-Header": "hello-world"}),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), "from-custom-headers/1.0.0"))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			}),
		)
		assert.NilError(t, err)
		_, err = c.Ping(t.Context(), PingOptions{})
		assert.NilError(t, err)
		assert.NilError(t, c.Close())
	})
	t.Run("no user-agent set", func(t *testing.T) {
		c, err := New(
			WithHTTPHeaders(map[string]string{"Other-Header": "hello-world"}),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), ""))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			}),
		)
		assert.NilError(t, err)
		_, err = c.Ping(t.Context(), PingOptions{})
		assert.NilError(t, err)
		assert.NilError(t, c.Close())
	})
	t.Run("reset custom user-agent", func(t *testing.T) {
		c, err := New(
			WithUserAgent(""),
			WithHTTPHeaders(map[string]string{"User-Agent": "from-custom-headers/1.0.0", "Other-Header": "hello-world"}),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), ""))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			}),
		)
		assert.NilError(t, err)
		_, err = c.Ping(t.Context(), PingOptions{})
		assert.NilError(t, err)
		assert.NilError(t, c.Close())
	})
}

func TestWithHTTPClient(t *testing.T) {
	cookieJar, err := cookiejar.New(nil)
	assert.NilError(t, err)
	pristineHTTPClient := func() *http.Client {
		return &http.Client{
			Timeout: 42 * time.Second,
			Jar:     cookieJar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{ServerName: "example.com", MinVersion: tls.VersionTLS12},
			},
		}
	}
	hc := pristineHTTPClient()
	_, err = New(WithHTTPClient(hc), WithHost("tcp://example.com:443"))
	assert.NilError(t, err)
	assert.DeepEqual(t, hc, pristineHTTPClient(),
		cmpopts.IgnoreUnexported(http.Transport{}, tls.Config{}),
		cmpopts.EquateComparable(&cookiejar.Jar{}))
}

func TestWithResponseHook(t *testing.T) {
	const hdrKey = "X-Test-Header"
	const hdrVal = "hello-world"

	t.Run("single hook", func(t *testing.T) {
		var got string
		c, err := New(
			WithResponseHook(func(resp *http.Response) error {
				got = resp.Header.Get(hdrKey)
				return nil
			}),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
				}
				resp.Header.Set(hdrKey, hdrVal)
				return resp, nil
			}),
		)
		assert.NilError(t, err)

		_, err = c.Ping(t.Context(), PingOptions{})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(got, hdrVal))

		assert.NilError(t, c.Close())
	})

	t.Run("invalid hook", func(t *testing.T) {
		_, err := New(WithResponseHook(nil))
		assert.Error(t, err, "invalid response hook: hook is nil")
	})

	t.Run("multiple hooks", func(t *testing.T) {
		var triggered []string

		c, err := New(
			WithResponseHook(func(*http.Response) error {
				triggered = append(triggered, "hook 1: "+hdrVal)
				return nil
			}),
			WithResponseHook(func(*http.Response) error {
				triggered = append(triggered, "hook 2: "+hdrVal)
				return nil
			}),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
				}
				resp.Header.Set(hdrKey, hdrVal)
				return resp, nil
			}),
		)
		assert.NilError(t, err)

		_, err = c.Ping(t.Context(), PingOptions{})
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(triggered, []string{"hook 1: " + hdrVal, "hook 2: " + hdrVal}))

		assert.NilError(t, c.Close())
	})

	t.Run("hook error", func(t *testing.T) {
		closed := false
		expError := errors.New("hook failed")

		c, err := New(
			WithResponseHook(func(*http.Response) error {
				return expError
			}),
			WithBaseMockClient(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       &closeTracker{onClose: func() { closed = true }},
				}, nil
			}),
		)
		assert.NilError(t, err)

		_, err = c.Ping(t.Context(), PingOptions{})
		assert.Check(t, is.ErrorIs(err, expError))
		assert.Check(t, closed)

		assert.NilError(t, c.Close())
	})
}

type closeTracker struct {
	onClose func()
}

func (c *closeTracker) Read(p []byte) (int, error) { return 0, io.EOF }

func (c *closeTracker) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return nil
}
