package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigListResult holds the result from the ConfigList method.
type ConfigListResult struct {
	Configs []swarm.Config
}

// ConfigList returns the list of configs.
func (cli *Client) ConfigList(ctx context.Context, options ConfigListOptions) (ConfigListResult, error) {
	var out ConfigListResult
	query := url.Values{}
	options.Filters.updateURLValues(query)

	resp, err := cli.get(ctx, "/configs", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ConfigListResult{}, err
	}

	err = json.NewDecoder(resp.Body).Decode(&out.Configs)
	if err != nil {
		return ConfigListResult{}, err
	}
	return out, nil
}
