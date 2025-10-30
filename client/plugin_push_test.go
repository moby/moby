package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPluginPushError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.PluginPush(context.Background(), "plugin_name", PluginPushOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.PluginPush(context.Background(), "", PluginPushOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.PluginPush(context.Background(), "    ", PluginPushOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestPluginPush(t *testing.T) {
	const expectedURL = "/plugins/plugin_name"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		auth := req.Header.Get(registry.AuthHeader)
		if auth != "authtoken" {
			return nil, fmt.Errorf("invalid auth header: expected 'authtoken', got %s", auth)
		}
		return mockResponse(http.StatusOK, nil, "")(req)
	}))
	assert.NilError(t, err)

	_, err = client.PluginPush(context.Background(), "plugin_name", PluginPushOptions{RegistryAuth: "authtoken"})
	assert.NilError(t, err)
}
