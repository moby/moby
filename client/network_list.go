package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/versions"
)

// NetworkList returns the list of networks configured in the docker host.
func (cli *Client) NetworkList(ctx context.Context, options NetworkListOptions) ([]network.Summary, error) {
	query := url.Values{}
	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return nil, err
		}
		if cli.version != "" && versions.LessThan(cli.version, "1.22") {
			legacyFormat, err := encodeLegacyFilters(filterJSON)
			if err != nil {
				return nil, err
			}
			filterJSON = legacyFormat
		}

		query.Set("filters", filterJSON)
	}
	var networkResources []network.Summary
	resp, err := cli.get(ctx, "/networks", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return networkResources, err
	}
	err = json.NewDecoder(resp.Body).Decode(&networkResources)
	return networkResources, err
}
