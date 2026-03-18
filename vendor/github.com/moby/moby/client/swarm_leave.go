package client

import (
	"context"
	"net/url"
)

// SwarmLeaveOptions contains options for leaving a swarm.
type SwarmLeaveOptions struct {
	Force bool
}

// SwarmLeaveResult represents the result of a SwarmLeave operation.
type SwarmLeaveResult struct{}

// SwarmLeave leaves the swarm.
func (cli *Client) SwarmLeave(ctx context.Context, options SwarmLeaveOptions) (SwarmLeaveResult, error) {
	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}
	resp, err := cli.post(ctx, "/swarm/leave", query, nil, nil)
	defer ensureReaderClosed(resp)
	return SwarmLeaveResult{}, err
}
