package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/plugin"
)

// PluginInspectOptions holds parameters to inspect a plugin.
type PluginInspectOptions struct {
	// Add future optional parameters here
}

// PluginInspectResult holds the result from the [Client.PluginInspect] method.
type PluginInspectResult struct {
	Plugin plugin.Plugin
	Raw    json.RawMessage
}

// PluginInspect inspects an existing plugin
func (cli *Client) PluginInspect(ctx context.Context, name string, options PluginInspectOptions) (PluginInspectResult, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return PluginInspectResult{}, err
	}
	resp, err := cli.get(ctx, "/plugins/"+name+"/json", nil, nil)
	if err != nil {
		return PluginInspectResult{}, err
	}

	var out PluginInspectResult
	out.Raw, err = decodeWithRaw(resp, &out.Plugin)
	return out, err
}
