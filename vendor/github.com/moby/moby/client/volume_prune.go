package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/volume"
)

// VolumePruneOptions holds parameters to prune volumes.
type VolumePruneOptions struct {
	// All controls whether named volumes should also be pruned. By
	// default, only anonymous volumes are pruned.
	All bool

	// Filters to apply when pruning.
	Filters Filters
}

// VolumePruneResult holds the result from the [Client.VolumePrune] method.
type VolumePruneResult struct {
	Report volume.PruneReport
}

// VolumePrune requests the daemon to delete unused data
func (cli *Client) VolumePrune(ctx context.Context, options VolumePruneOptions) (VolumePruneResult, error) {
	if options.All {
		if _, ok := options.Filters["all"]; ok {
			return VolumePruneResult{}, errdefs.ErrInvalidArgument.WithMessage(`conflicting options: cannot specify both "all" and "all" filter`)
		}
		if options.Filters == nil {
			options.Filters = Filters{}
		}
		options.Filters.Add("all", "true")
	}

	query := url.Values{}
	options.Filters.updateURLValues(query)

	resp, err := cli.post(ctx, "/volumes/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumePruneResult{}, err
	}

	var report volume.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return VolumePruneResult{}, fmt.Errorf("error retrieving volume prune report: %v", err)
	}

	return VolumePruneResult{Report: report}, nil
}
