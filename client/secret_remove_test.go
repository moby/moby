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

func TestSecretRemoveUnsupported(t *testing.T) {
	client, err := NewClientWithOpts(WithVersion("1.24"), WithHTTPClient(&http.Client{}))
	assert.NilError(t, err)
	err = client.SecretRemove(context.Background(), "secret_id")
	assert.Check(t, is.Error(err, `"secret remove" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(WithVersion("1.25"), WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	err = client.SecretRemove(context.Background(), "secret_id")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.SecretRemove(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.SecretRemove(context.Background(), "   ")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestSecretRemove(t *testing.T) {
	expectedURL := "/v1.25/secrets/secret_id"

	client, err := NewClientWithOpts(WithVersion("1.25"), WithMockClient(func(req *http.Request) (*http.Response, error) {
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

	err = client.SecretRemove(context.Background(), "secret_id")
	assert.NilError(t, err)
}
