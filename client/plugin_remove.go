package client

import (
	"context"
	"net/url"
)

// PluginRemoveOptions holds parameters to remove plugins.
type PluginRemoveOptions struct {
	Force bool
}

// PluginRemoveResult represents the result of a plugin removal.
type PluginRemoveResult struct {
	// Currently empty; can be extended in the future if needed.
}

// PluginRemove removes a plugin
func (cli *Client) PluginRemove(ctx context.Context, name string, options PluginRemoveOptions) (PluginRemoveResult, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return PluginRemoveResult{}, err
	}

	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.delete(ctx, "/plugins/"+name, query, nil)
	defer ensureReaderClosed(resp)
	return PluginRemoveResult{}, err
}
