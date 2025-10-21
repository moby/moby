package client

import (
	"context"

	"github.com/moby/moby/api/types/plugin"
)

// PluginInspectOptions holds parameters to inspect a plugin.
type PluginInspectOptions struct {
	// Add future optional parameters here
}

// PluginInspectResult holds the result from the [Client.PluginInspect] method.
type PluginInspectResult struct {
	Raw    []byte
	Plugin plugin.Plugin
}

// PluginInspect inspects an existing plugin
func (cli *Client) PluginInspect(ctx context.Context, name string, options PluginInspectOptions) (PluginInspectResult, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return PluginInspectResult{}, err
	}
	resp, err := cli.get(ctx, "/plugins/"+name+"/json", nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return PluginInspectResult{}, err
	}

	var out PluginInspectResult
	out.Raw, err = decodeWithRaw(resp, &out.Plugin)
	return out, err
}
