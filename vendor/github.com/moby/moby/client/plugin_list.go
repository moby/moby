package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/plugin"
)

// PluginList returns the installed plugins
func (cli *Client) PluginList(ctx context.Context, filter Filters) (plugin.ListResponse, error) {
	var plugins plugin.ListResponse
	query := url.Values{}

	filter.updateURLValues(query)
	resp, err := cli.get(ctx, "/plugins", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return plugins, err
	}

	err = json.NewDecoder(resp.Body).Decode(&plugins)
	return plugins, err
}
