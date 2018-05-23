package client // import "github.com/docker/docker/client"

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
	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.delete(ctx, "/plugins/"+name, query, nil)
	ensureReaderClosed(resp)
	return wrapResponseError(err, resp, "plugin", name)
}
