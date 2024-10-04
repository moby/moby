package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types/filters"
)

// ContainersPrune requests the daemon to delete unused data
func (cli *Client) ContainersPrune(ctx context.Context, pruneFilters filters.Args, dryRun bool) (types.ContainersPruneReport, error) {
	var report types.ContainersPruneReport

	if err := cli.NewVersionError(ctx, "1.25", "container prune"); err != nil {
		return report, err
	}

	filtersQuery, err := getFiltersQuery(pruneFilters)
	if err != nil {
		return report, err
	}

	// add dryRun option to query URL
	query, err := setdryRunQuery(dryRun, filtersQuery)
	if err != nil {
		return report, err
	}

	// dry run prune will have
	// /containers/prune?dryRun=true
	serverResp, err := cli.post(ctx, "/containers/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return report, err
	}

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
