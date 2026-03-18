package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ServiceListOptions holds parameters to list services with.
type ServiceListOptions struct {
	Filters Filters

	// Status indicates whether the server should include the service task
	// count of running and desired tasks.
	Status bool
}

// ServiceListResult represents the result of a service list operation.
type ServiceListResult struct {
	Items []swarm.Service
}

// ServiceList returns the list of services.
func (cli *Client) ServiceList(ctx context.Context, options ServiceListOptions) (ServiceListResult, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)

	if options.Status {
		query.Set("status", "true")
	}

	resp, err := cli.get(ctx, "/services", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ServiceListResult{}, err
	}

	var services []swarm.Service
	err = json.NewDecoder(resp.Body).Decode(&services)
	return ServiceListResult{Items: services}, err
}
