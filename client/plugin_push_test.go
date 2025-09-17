package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPluginPushError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.PluginPush(context.Background(), "plugin_name", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.PluginPush(context.Background(), "", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.PluginPush(context.Background(), "    ", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestPluginPush(t *testing.T) {
	const expectedURL = "/plugins/plugin_name"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		auth := req.Header.Get(registry.AuthHeader)
		if auth != "authtoken" {
			return nil, fmt.Errorf("invalid auth header: expected 'authtoken', got %s", auth)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}))
	assert.NilError(t, err)

	_, err = client.PluginPush(context.Background(), "plugin_name", "authtoken")
	assert.NilError(t, err)
}
