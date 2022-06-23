package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestServiceRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), "service_id")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestServiceRemoveNotFoundError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "no such service: service_id"))),
	)
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), "service_id")
	assert.ErrorContains(t, err, "no such service: service_id")
	assert.Check(t, IsErrNotFound(err))
}

func TestServiceRemove(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/services/service_id"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
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
		})),
	)
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), "service_id")
	if err != nil {
		t.Fatal(err)
	}
}
