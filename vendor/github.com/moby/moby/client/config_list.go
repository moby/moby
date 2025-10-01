package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/filters"
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

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return out, err
		}

		query.Set("filters", filterJSON)
	}

	resp, err := cli.get(ctx, "/configs", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return out, err
	}

	err = json.NewDecoder(resp.Body).Decode(&out.Configs)
	return out, err
}
