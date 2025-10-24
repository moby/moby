package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ServiceInspectOptions holds parameters related to the service inspect operation.
type ServiceInspectOptions struct {
	InsertDefaults bool
}

// ServiceInspectResult represents the result of a service inspect operation.
type ServiceInspectResult struct {
	Service swarm.Service
	Raw     json.RawMessage
}

// ServiceInspect retrieves detailed information about a specific service by its ID.
func (cli *Client) ServiceInspect(ctx context.Context, serviceID string, options ServiceInspectOptions) (ServiceInspectResult, error) {
	serviceID, err := trimID("service", serviceID)
	if err != nil {
		return ServiceInspectResult{}, err
	}

	query := url.Values{}
	query.Set("insertDefaults", fmt.Sprintf("%v", options.InsertDefaults))
	resp, err := cli.get(ctx, "/services/"+serviceID, query, nil)
	if err != nil {
		return ServiceInspectResult{}, err
	}

	var out ServiceInspectResult
	out.Raw, err = decodeWithRaw(resp, &out.Service)
	return out, err
}
