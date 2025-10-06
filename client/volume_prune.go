package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/volume"
)

// VolumesPrune requests the daemon to delete unused data
func (cli *Client) VolumesPrune(ctx context.Context, pruneFilters Filters) (volume.PruneReport, error) {
	query := url.Values{}
	pruneFilters.updateURLValues(query)

	resp, err := cli.post(ctx, "/volumes/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return volume.PruneReport{}, err
	}

	var report volume.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return volume.PruneReport{}, fmt.Errorf("Error retrieving volume prune report: %v", err)
	}

	return report, nil
}
