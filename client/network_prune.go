package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/network"
)

// NetworkPruneOptions holds parameters to prune networks.
type NetworkPruneOptions struct {
	Filters Filters

	// OnProgress receives successful deletions as they occur. Requires API v1.56.
	// Returning an error cancels the request.
	OnProgress func(PruneProgress) error
}

// NetworkPruneResult holds the result from the [Client.NetworkPrune] method.
type NetworkPruneResult struct {
	Report network.PruneReport
}

// NetworkPrune requests the daemon to delete unused networks
func (cli *Client) NetworkPrune(ctx context.Context, opts NetworkPruneOptions) (NetworkPruneResult, error) {
	query := url.Values{}
	opts.Filters.updateURLValues(query)

	var report network.PruneReport
	if err := cli.prune(ctx, "/networks/prune", query, opts.OnProgress, &report); err != nil {
		return NetworkPruneResult{}, fmt.Errorf("error retrieving network prune report: %w", err)
	}

	return NetworkPruneResult{Report: report}, nil
}
