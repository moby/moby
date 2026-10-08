package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/image"
)

// ImagePruneOptions holds parameters to prune images.
type ImagePruneOptions struct {
	Filters Filters

	// OnProgress receives successful deletions and untagging as they occur.
	// Requires API v1.56. Returning an error cancels the request.
	OnProgress func(PruneProgress) error
}

// ImagePruneResult holds the result from the [Client.ImagePrune] method.
type ImagePruneResult struct {
	Report image.PruneReport
}

// ImagePrune requests the daemon to delete unused data
func (cli *Client) ImagePrune(ctx context.Context, opts ImagePruneOptions) (ImagePruneResult, error) {
	query := url.Values{}
	opts.Filters.updateURLValues(query)

	var report image.PruneReport
	if err := cli.prune(ctx, "/images/prune", query, opts.OnProgress, &report); err != nil {
		return ImagePruneResult{}, fmt.Errorf("error retrieving image prune report: %w", err)
	}

	return ImagePruneResult{Report: report}, nil
}
