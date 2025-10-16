package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/volume"
)

// VolumePruneOptions holds parameters to prune networks.
type VolumePruneOptions struct {
	Filters Filters
}

// VolumePruneResult holds the result from the [Client.VolumesPrune] method.
type VolumePruneResult struct {
	Report volume.PruneReport
}

// VolumesPrune requests the daemon to delete unused data
func (cli *Client) VolumesPrune(ctx context.Context, opts VolumePruneOptions) (VolumePruneResult, error) {
	query := url.Values{}
	opts.Filters.updateURLValues(query)

	resp, err := cli.post(ctx, "/volumes/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumePruneResult{}, err
	}

	var report volume.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return VolumePruneResult{}, fmt.Errorf("Error retrieving volume prune report: %v", err)
	}

	return VolumePruneResult{Report: report}, nil
}
