package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/swarm"
)

// ConfigList returns the list of configs.
func (cli *Client) ConfigList(ctx context.Context, options ConfigListOptions) ([]swarm.Config, error) {
	query := url.Values{}

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return nil, err
		}

		query.Set("filters", filterJSON)
	}

	resp, err := cli.get(ctx, "/configs", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var configs []swarm.Config
	err = json.NewDecoder(resp.Body).Decode(&configs)
	return configs, err
}
