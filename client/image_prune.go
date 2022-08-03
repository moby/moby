package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

// ImagesPrune requests the daemon to delete unused data
func (cli *Client) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (types.ImagesPruneReport, error) {
	var report types.ImagesPruneReport

	versioned, err := cli.versioned(ctx)
	if err != nil {
		return report, err
	}
	if err := versioned.NewVersionError("1.25", "image prune"); err != nil {
		return report, err
	}

	query, err := getFiltersQuery(pruneFilters)
	if err != nil {
		return report, err
	}

	serverResp, err := versioned.post(ctx, "/images/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return report, err
	}

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
