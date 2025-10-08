package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/network"
)

// NetworksPrune requests the daemon to delete unused networks
func (cli *Client) NetworksPrune(ctx context.Context, pruneFilters Filters) (network.PruneReport, error) {
	query := url.Values{}
	pruneFilters.updateURLValues(query)

	resp, err := cli.post(ctx, "/networks/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return network.PruneReport{}, err
	}

	var report network.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return network.PruneReport{}, fmt.Errorf("Error retrieving network prune report: %v", err)
	}

	return report, nil
}
