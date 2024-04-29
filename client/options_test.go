package client

import (
	"context"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestOptionWithHostFromEnv(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	c, err := NewClientWithOpts(ctx, WithHostFromEnv())
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

	c, err = NewClientWithOpts(ctx, WithHostFromEnv())
	assert.NilError(t, err)
	assert.Check(t, c.client != nil)
	assert.Check(t, is.Equal(c.basePath, "/test/"))
	assert.Check(t, is.Equal(c.host, "tcp://foo.example.com:2376/test/"))
	assert.Check(t, is.Equal(c.proto, "tcp"))
	assert.Check(t, is.Equal(c.addr, "foo.example.com:2376"))
}

func TestOptionWithTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	timeout := 10 * time.Second
	c, err := NewClientWithOpts(ctx, WithTimeout(timeout))
	assert.NilError(t, err)
	assert.Check(t, c.client != nil)
	assert.Equal(t, c.client.Timeout, timeout)
}

func TestOptionWithVersionFromEnv(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	c, err := NewClientWithOpts(ctx, WithVersionFromEnv())
	assert.NilError(t, err)
	assert.Check(t, c.client != nil)
	assert.Equal(t, c.version, api.DefaultVersion)
	assert.Equal(t, c.manualOverride, false)

	t.Setenv("DOCKER_API_VERSION", "2.9999")

	c, err = NewClientWithOpts(ctx, WithVersionFromEnv())
	assert.NilError(t, err)
	assert.Check(t, c.client != nil)
	assert.Equal(t, c.version, "2.9999")
	assert.Equal(t, c.manualOverride, true)
}

func TestWithUserAgent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	const userAgent = "Magic-Client/v1.2.3"
	t.Run("user-agent", func(t *testing.T) {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()

		c, err := NewClientWithOpts(
			ctx2,
			WithUserAgent(userAgent),
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), userAgent))
				return &http.Response{StatusCode: http.StatusOK}, nil
			})),
		)
		assert.Check(t, err)
		_, err = c.Ping(context.Background())
		assert.Check(t, err)
		assert.Check(t, c.Close(ctx2))
	})
	t.Run("user-agent and custom headers", func(t *testing.T) {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()

		c, err := NewClientWithOpts(
			ctx2,
			WithUserAgent(userAgent),
			WithHTTPHeaders(map[string]string{"User-Agent": "should-be-ignored/1.0.0", "Other-Header": "hello-world"}),
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), userAgent))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			})),
		)
		assert.Check(t, err)
		_, err = c.Ping(context.Background())
		assert.Check(t, err)
		assert.Check(t, c.Close(ctx2))
	})
	t.Run("custom headers", func(t *testing.T) {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()

		c, err := NewClientWithOpts(
			ctx2,
			WithHTTPHeaders(map[string]string{"User-Agent": "from-custom-headers/1.0.0", "Other-Header": "hello-world"}),
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), "from-custom-headers/1.0.0"))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			})),
		)
		assert.Check(t, err)
		_, err = c.Ping(ctx2)
		assert.Check(t, err)
		assert.Check(t, c.Close(ctx2))
	})
	t.Run("no user-agent set", func(t *testing.T) {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()

		c, err := NewClientWithOpts(
			ctx2,
			WithHTTPHeaders(map[string]string{"Other-Header": "hello-world"}),
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), ""))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			})),
		)
		assert.Check(t, err)
		_, err = c.Ping(ctx2)
		assert.Check(t, err)
		assert.Check(t, c.Close(ctx2))
	})
	t.Run("reset custom user-agent", func(t *testing.T) {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()

		c, err := NewClientWithOpts(
			ctx2,
			WithUserAgent(""),
			WithHTTPHeaders(map[string]string{"User-Agent": "from-custom-headers/1.0.0", "Other-Header": "hello-world"}),
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, is.Equal(req.Header.Get("User-Agent"), ""))
				assert.Check(t, is.Equal(req.Header.Get("Other-Header"), "hello-world"))
				return &http.Response{StatusCode: http.StatusOK}, nil
			})),
		)
		assert.Check(t, err)
		_, err = c.Ping(ctx2)
		assert.Check(t, err)
		assert.Check(t, c.Close(ctx2))
	})
}
