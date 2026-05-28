package client

import (
	"context"
	"io"
	"net/http"

	"github.com/moby/moby/api/types/registry"
)

// PluginPushOptions holds parameters to push a plugin.
type PluginPushOptions struct {
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry
}

// PluginPushResult is the result of a plugin push operation
type PluginPushResult struct {
	io.ReadCloser
}

// PluginPush pushes a plugin to a registry
func (cli *Client) PluginPush(ctx context.Context, name string, options PluginPushOptions) (PluginPushResult, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return PluginPushResult{}, err
	}
	resp, err := cli.post(ctx, "/plugins/"+name+"/push", nil, nil, http.Header{
		registry.AuthHeader: {options.RegistryAuth},
	})
	if err != nil {
		return PluginPushResult{}, err
	}
	return PluginPushResult{resp.Body}, nil
}
