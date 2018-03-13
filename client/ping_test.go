package client // import "github.com/docker/docker/client"

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"golang.org/x/net/context"
)

// TestPingFail tests that when a server sends a non-successful response that we
// can still grab API details, when set.
// Some of this is just excercising the code paths to make sure there are no
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
			}
			resp.Body = ioutil.NopCloser(strings.NewReader("some error with the server"))
			return resp, nil
		}),
	}

	ping, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, ""))
	assert.Check(t, is.Equal(false, ping.Experimental))
	assert.Check(t, is.Equal("", ping.APIVersion))

	withHeader = true
	ping2, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, ""))
	assert.Check(t, is.Equal(true, ping2.Experimental))
	assert.Check(t, is.Equal("awesome", ping2.APIVersion))
}

// TestPingWithError tests the case where there is a protocol error in the ping.
// This test is mostly just testing that there are no panics in this code path.
func TestPingWithError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusInternalServerError}
			resp.Header = http.Header{}
			resp.Header.Set("API-Version", "awesome")
			resp.Header.Set("Docker-Experimental", "true")
			resp.Body = ioutil.NopCloser(strings.NewReader("some error with the server"))
			return resp, errors.New("some error")
		}),
	}

	ping, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, ""))
	assert.Check(t, is.Equal(false, ping.Experimental))
	assert.Check(t, is.Equal("", ping.APIVersion))
}

// TestPingSuccess tests that we are able to get the expected API headers/ping
// details on success.
func TestPingSuccess(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			resp := &http.Response{StatusCode: http.StatusInternalServerError}
			resp.Header = http.Header{}
			resp.Header.Set("API-Version", "awesome")
			resp.Header.Set("Docker-Experimental", "true")
			resp.Body = ioutil.NopCloser(strings.NewReader("some error with the server"))
			return resp, nil
		}),
	}
	ping, err := client.Ping(context.Background())
	assert.Check(t, is.ErrorContains(err, ""))
	assert.Check(t, is.Equal(true, ping.Experimental))
	assert.Check(t, is.Equal("awesome", ping.APIVersion))
}
