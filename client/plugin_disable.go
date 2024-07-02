package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"

	"github.com/docker/docker/api/types/plugin"
)

// PluginDisable disables a plugin
func (cli *Client) PluginDisable(ctx context.Context, name string, options plugin.DisableOptions) error {
	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}
	resp, err := cli.post(ctx, "/plugins/"+name+"/disable", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
