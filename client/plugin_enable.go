package client

import (
	"context"
	"net/url"
	"strconv"
)

// PluginEnableOptions holds parameters to enable plugins.
type PluginEnableOptions struct {
	Timeout int
}

// PluginEnableResult represents the result of a plugin enable operation.
type PluginEnableResult struct {
	// Currently empty; can be extended in the future if needed.
}

// PluginEnable enables a plugin
func (cli *Client) PluginEnable(ctx context.Context, name string, options PluginEnableOptions) (PluginEnableResult, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return PluginEnableResult{}, err
	}
	query := url.Values{}
	query.Set("timeout", strconv.Itoa(options.Timeout))

	resp, err := cli.post(ctx, "/plugins/"+name+"/enable", query, nil, nil)
	defer ensureReaderClosed(resp)
	return PluginEnableResult{}, err
}
