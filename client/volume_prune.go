package client

import (
	"context"
	"fmt"
	"net/url"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/volume"
)

// VolumePruneOptions holds parameters to prune volumes.
type VolumePruneOptions struct {
	// All controls whether named volumes should also be pruned. By
	// default, only anonymous volumes are pruned.
	All bool

	// Filters to apply when pruning.
	Filters Filters

	// OnProgress receives successful deletions as they occur. Requires API v1.56.
	// Returning an error cancels the request.
	OnProgress func(PruneProgress) error
}

// VolumePruneResult holds the result from the [Client.VolumePrune] method.
type VolumePruneResult struct {
	Report volume.PruneReport
}

// VolumePrune requests the daemon to delete unused data
func (cli *Client) VolumePrune(ctx context.Context, options VolumePruneOptions) (VolumePruneResult, error) {
	if options.All {
		if _, ok := options.Filters["all"]; ok {
			return VolumePruneResult{}, cerrdefs.ErrInvalidArgument.WithMessage(`conflicting options: cannot specify both "all" and "all" filter`)
		}
		if options.Filters == nil {
			options.Filters = Filters{}
		}
		options.Filters.Add("all", "true")
	}

	query := url.Values{}
	options.Filters.updateURLValues(query)

	var report volume.PruneReport
	if err := cli.prune(ctx, "/volumes/prune", query, options.OnProgress, &report); err != nil {
		return VolumePruneResult{}, fmt.Errorf("error retrieving volume prune report: %w", err)
	}

	return VolumePruneResult{Report: report}, nil
}
