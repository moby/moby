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

func TestConfigRemoveUnsupported(t *testing.T) {
	client := &Client{
		version: "1.29",
		client:  &http.Client{},
	}
	err := client.ConfigRemove(context.Background(), "config_id")
	assert.Check(t, is.Error(err, `"config remove" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestConfigRemoveError(t *testing.T) {
	client := &Client{
		version: "1.30",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.ConfigRemove(context.Background(), "config_id")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ConfigRemove(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ConfigRemove(context.Background(), "    ")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestConfigRemove(t *testing.T) {
	expectedURL := "/v1.30/configs/config_id"

	client := &Client{
		version: "1.30",
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

	err := client.ConfigRemove(context.Background(), "config_id")
	assert.NilError(t, err)
}
