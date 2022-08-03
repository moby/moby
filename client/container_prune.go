package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

// ContainersPrune requests the daemon to delete unused data
func (cli *Client) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (types.ContainersPruneReport, error) {
	var report types.ContainersPruneReport

	versioned, err := cli.versioned(ctx)
	if err != nil {
		return report, err
	}
	if err := versioned.NewVersionError("1.25", "container prune"); err != nil {
		return report, err
	}

	query, err := getFiltersQuery(pruneFilters)
	if err != nil {
		return report, err
	}

	serverResp, err := versioned.post(ctx, "/containers/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return report, err
	}

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
