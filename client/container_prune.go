package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/container"
)

// ContainersPrune requests the daemon to delete unused data
func (cli *Client) ContainersPrune(ctx context.Context, pruneFilters Filters) (container.PruneReport, error) {
	query := url.Values{}
	pruneFilters.updateURLValues(query)

	resp, err := cli.post(ctx, "/containers/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return container.PruneReport{}, err
	}

	var report container.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return container.PruneReport{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
