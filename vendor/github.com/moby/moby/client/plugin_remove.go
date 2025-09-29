package client

import (
	"context"
	"net/url"
)

// PluginRemoveOptions holds parameters to remove plugins.
type PluginRemoveOptions struct {
	Force bool
}

// PluginRemove removes a plugin
func (cli *Client) PluginRemove(ctx context.Context, name string, options PluginRemoveOptions) error {
	name, err := trimID("plugin", name)
	if err != nil {
		return err
	}

	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.delete(ctx, "/plugins/"+name, query, nil)
	defer ensureReaderClosed(resp)
	return err
}
