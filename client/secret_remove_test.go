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
	client := &Client{
		version: "1.24",
		client:  &http.Client{},
	}
	err := client.SecretRemove(context.Background(), "secret_id")
	assert.Check(t, is.Error(err, `"secret remove" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretRemoveError(t *testing.T) {
	client := &Client{
		version: "1.25",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.SecretRemove(context.Background(), "secret_id")
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

	client := &Client{
		version: "1.25",
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

	err := client.SecretRemove(context.Background(), "secret_id")
	assert.NilError(t, err)
}
