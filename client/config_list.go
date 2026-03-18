package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigListOptions holds parameters to list configs
type ConfigListOptions struct {
	Filters Filters
}

// ConfigListResult holds the result from the [client.ConfigList] method.
type ConfigListResult struct {
	Items []swarm.Config
}

// ConfigList returns the list of configs.
func (cli *Client) ConfigList(ctx context.Context, options ConfigListOptions) (ConfigListResult, error) {
	query := url.Values{}
	options.Filters.updateURLValues(query)

	resp, err := cli.get(ctx, "/configs", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ConfigListResult{}, err
	}

	var out ConfigListResult
	err = json.NewDecoder(resp.Body).Decode(&out.Items)
	if err != nil {
		return ConfigListResult{}, err
	}
	return out, nil
}
