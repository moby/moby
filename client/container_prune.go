package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/container"
)

// ContainerPruneOptions holds parameters to prune containers.
type ContainerPruneOptions struct {
	Filters Filters

	// OnProgress receives successful deletions as they occur. Requires API v1.56.
	// Returning an error stops reading the stream and cancels the request.
	// The daemon may return only the final report, for example when response
	// authorization is enabled; in that case OnProgress is not called.
	OnProgress func(PruneProgress) error
}

// ContainerPruneResult holds the result from the [Client.ContainerPrune] method.
type ContainerPruneResult struct {
	Report container.PruneReport
}

// ContainerPrune requests the daemon to delete unused data
func (cli *Client) ContainerPrune(ctx context.Context, opts ContainerPruneOptions) (ContainerPruneResult, error) {
	query := url.Values{}
	opts.Filters.updateURLValues(query)

	var report container.PruneReport
	if err := cli.prune(ctx, "/containers/prune", query, opts.OnProgress, &report); err != nil {
		return ContainerPruneResult{}, fmt.Errorf("error retrieving container prune report: %w", err)
	}

	return ContainerPruneResult{Report: report}, nil
}
