package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestServiceRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), "service_id")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ServiceRemove(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ServiceRemove(context.Background(), "    ")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestServiceRemoveNotFoundError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusNotFound, "no such service: service_id")))
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), "service_id")
	assert.Check(t, is.ErrorContains(err, "no such service: service_id"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestServiceRemove(t *testing.T) {
	expectedURL := "/services/service_id"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
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
	}))
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), "service_id")
	assert.NilError(t, err)
}
