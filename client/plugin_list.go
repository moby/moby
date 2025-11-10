package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/plugin"
)

// PluginListOptions holds parameters to list plugins.
type PluginListOptions struct {
	Filters Filters
}

// PluginListResult represents the result of a plugin list operation.
type PluginListResult struct {
	Items []plugin.Plugin
}

// PluginList returns the installed plugins
func (cli *Client) PluginList(ctx context.Context, options PluginListOptions) (PluginListResult, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)
	resp, err := cli.get(ctx, "/plugins", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return PluginListResult{}, err
	}

	var plugins plugin.ListResponse
	err = json.NewDecoder(resp.Body).Decode(&plugins)
	return PluginListResult{Items: plugins}, err
}
