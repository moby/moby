package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"
	"strconv"
)

// PluginEnableOptions holds parameters to enable plugins.
type PluginEnableOptions struct {
	Timeout int
}

// PluginEnable enables a plugin
func (cli *Client) PluginEnable(ctx context.Context, name string, options PluginEnableOptions) error {
	query := url.Values{}
	query.Set("timeout", strconv.Itoa(options.Timeout))

	resp, err := cli.post(ctx, "/plugins/"+name+"/enable", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
