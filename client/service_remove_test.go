package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestServiceRemoveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.ServiceRemove(context.Background(), "service_id")
	assert.EqualError(t, err, "Error response from daemon: Server error")
}

func TestServiceRemoveNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "missing")),
	}

	err := client.ServiceRemove(context.Background(), "service_id")
	assert.EqualError(t, err, "Error: No such service: service_id")
	assert.True(t, IsErrNotFound(err))
}

func TestServiceRemove(t *testing.T) {
	expectedURL := "/services/service_id"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != "DELETE" {
				return nil, fmt.Errorf("expected DELETE method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	}

	err := client.ServiceRemove(context.Background(), "service_id")
	if err != nil {
		t.Fatal(err)
	}
}
