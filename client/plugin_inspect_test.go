package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/plugin"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPluginInspectError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.PluginInspect(t.Context(), "nothing", PluginInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestPluginInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.PluginInspect(t.Context(), "", PluginInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.PluginInspect(t.Context(), "    ", PluginInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestPluginInspect(t *testing.T) {
	const expectedURL = "/plugins/plugin_name"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		content, err := json.Marshal(plugin.Plugin{
			ID: "plugin_id",
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(content)),
		}, nil
	}))
	assert.NilError(t, err)

	resp, err := client.PluginInspect(t.Context(), "plugin_name", PluginInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.Plugin.ID, "plugin_id"))
}
