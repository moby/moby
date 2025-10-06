package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ServiceList returns the list of services.
func (cli *Client) ServiceList(ctx context.Context, options ServiceListOptions) ([]swarm.Service, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)

	if options.Status {
		query.Set("status", "true")
	}

	resp, err := cli.get(ctx, "/services", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var services []swarm.Service
	err = json.NewDecoder(resp.Body).Decode(&services)
	return services, err
}
