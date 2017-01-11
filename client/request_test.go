package client

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// TestSetHostHeader should set fake host for local communications, set real host
// for normal communications.
func TestSetHostHeader(t *testing.T) {
	testURL := "/test"
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

	for c, test := range testCases {
		proto, addr, basePath, err := ParseHost(test.host)
		if err != nil {
			t.Fatal(err)
		}

		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, testURL) {
					return nil, fmt.Errorf("Test Case #%d: Expected URL %q, got %q", c, testURL, req.URL)
				}
				if req.Host != test.expectedHost {
					return nil, fmt.Errorf("Test Case #%d: Expected host %q, got %q", c, test.expectedHost, req.Host)
				}
				if req.URL.Host != test.expectedURLHost {
					return nil, fmt.Errorf("Test Case #%d: Expected URL host %q, got %q", c, test.expectedURLHost, req.URL.Host)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader(([]byte("")))),
				}, nil
			}),

			proto:    proto,
			addr:     addr,
			basePath: basePath,
		}

		_, err = client.sendRequest(context.Background(), "GET", testURL, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
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
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

// TestIsDeletedSuccessfully tests that whether the client failed to send
// the delete or the response status code is "301" an error message is returned
func TestIsDeletedSuccessfully(t *testing.T) {
	tt := []struct {
		cliErr error
		resp   serverResponse
		item   string
		expErr error
	}{
		{nil, serverResponse{nil, http.Header{}, 204}, "id", nil},
		{nil, serverResponse{nil, http.Header{}, 200}, "id", nil},
		{errors.New("hl"), serverResponse{nil, http.Header{}, 204}, "id", errors.New("hl")},
		{nil, serverResponse{nil, http.Header{}, 301}, "sid", errors.New("Bad name: sid")},
	}

	for i, te := range tt {
		if err := isDeletedSuccessfully(te.resp, te.item, te.cliErr); !reflect.DeepEqual(err, te.expErr) {
			t.Errorf("test #%d: expected error to be '%s' but got '%s'", i, te.expErr, err)
		}
	}
}
