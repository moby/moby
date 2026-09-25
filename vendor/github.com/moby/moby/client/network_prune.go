package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/network"
)

// NetworkPruneOptions holds parameters to prune networks.
type NetworkPruneOptions struct {
	Filters Filters
}

// NetworkPruneResult holds the result from the [Client.NetworkPrune] method.
type NetworkPruneResult struct {
	Report network.PruneReport
}

// NetworkPrune requests the daemon to delete unused networks
func (cli *Client) NetworkPrune(ctx context.Context, opts NetworkPruneOptions) (NetworkPruneResult, error) {
	query := url.Values{}
	opts.Filters.updateURLValues(query)

	resp, err := cli.post(ctx, "/networks/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return NetworkPruneResult{}, err
	}

	var report network.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return NetworkPruneResult{}, fmt.Errorf("Error retrieving network prune report: %v", err)
	}

	return NetworkPruneResult{Report: report}, nil
}
