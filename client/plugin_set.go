package client

import (
	"context"
)

// PluginSetOptions defines options for modifying a plugin's settings.
type PluginSetOptions struct {
	Args []string
}

// PluginSetResult represents the result of a plugin set operation.
type PluginSetResult struct {
	// Currently empty; can be extended in the future if needed.
}

// PluginSet modifies settings for an existing plugin
func (cli *Client) PluginSet(ctx context.Context, name string, options PluginSetOptions) (PluginSetResult, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return PluginSetResult{}, err
	}

	resp, err := cli.post(ctx, "/plugins/"+name+"/set", nil, options.Args, nil)
	defer ensureReaderClosed(resp)
	return PluginSetResult{}, err
}
