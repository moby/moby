package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// VolumesPrune requests the daemon to delete unused data
func (cli *Client) VolumesPrune(ctx context.Context, pruneFilters filters.Args) (volume.PruneReport, error) {
	var report volume.PruneReport

	if err := cli.NewVersionError(ctx, "1.25", "volume prune"); err != nil {
		return report, err
	}

	query, err := getFiltersQuery(pruneFilters)
	if err != nil {
		return report, err
	}

	serverResp, err := cli.post(ctx, "/volumes/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return report, err
	}

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving volume prune report: %v", err)
	}

	return report, nil
}
