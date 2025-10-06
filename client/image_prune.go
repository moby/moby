package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/image"
)

// ImagesPrune requests the daemon to delete unused data
func (cli *Client) ImagesPrune(ctx context.Context, pruneFilters Filters) (image.PruneReport, error) {
	query := url.Values{}
	pruneFilters.updateURLValues(query)

	resp, err := cli.post(ctx, "/images/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return image.PruneReport{}, err
	}

	var report image.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return image.PruneReport{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
