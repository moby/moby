package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestServiceRemoveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.ServiceRemove(context.Background(), "service_id")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestServiceRemoveNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "no such service: service_id")),
	}

	err := client.ServiceRemove(context.Background(), "service_id")
	assert.Check(t, is.ErrorContains(err, "no such service: service_id"))
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

func TestServiceRemove(t *testing.T) {
	expectedURL := "/services/service_id"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodDelete {
				return nil, fmt.Errorf("expected DELETE method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	}

	err := client.ServiceRemove(context.Background(), "service_id")
	if err != nil {
		t.Fatal(err)
	}
}
