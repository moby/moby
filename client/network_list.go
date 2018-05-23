package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
)

// NetworkListOptions holds parameters to filter the list of networks with.
type NetworkListOptions struct {
	Filters filters.Args
}

// NetworkList returns the list of networks configured in the docker host.
func (cli *Client) NetworkList(ctx context.Context, options NetworkListOptions) ([]network.Resource, error) {
	query := url.Values{}
	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToParamWithVersion(cli.version, options.Filters)
		if err != nil {
			return nil, err
		}

		query.Set("filters", filterJSON)
	}
	var networkResources []network.Resource
	resp, err := cli.get(ctx, "/networks", query, nil)
	if err != nil {
		return networkResources, err
	}
	err = json.NewDecoder(resp.body).Decode(&networkResources)
	ensureReaderClosed(resp)
	return networkResources, err
}
