package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
)

// ImagesPrune requests the daemon to delete unused data
func (cli *Client) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (image.PruneReport, error) {
	if err := cli.NewVersionError(ctx, "1.25", "image prune"); err != nil {
		return image.PruneReport{}, err
	}

	query, err := getFiltersQuery(pruneFilters)
	if err != nil {
		return image.PruneReport{}, err
	}

	serverResp, err := cli.post(ctx, "/images/prune", query, nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return image.PruneReport{}, err
	}

	var report image.PruneReport
	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return image.PruneReport{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
