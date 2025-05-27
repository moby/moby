package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

// ServiceList returns the list of services.
func (cli *Client) ServiceList(ctx context.Context, options swarm.ServiceListOptions) ([]swarm.Service, error) {
	query := url.Values{}

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return nil, err
		}

		query.Set("filters", filterJSON)
	}

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
