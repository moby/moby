package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/image"
)

// ImagePruneOptions holds parameters to prune images.
type ImagePruneOptions struct {
	Filters Filters
}

// ImagePruneResult holds the result from the [Client.ImagePrune] method.
type ImagePruneResult struct {
	Report image.PruneReport
}

// ImagePrune requests the daemon to delete unused data
func (cli *Client) ImagePrune(ctx context.Context, opts ImagePruneOptions) (ImagePruneResult, error) {
	query := url.Values{}
	opts.Filters.updateURLValues(query)

	resp, err := cli.post(ctx, "/images/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ImagePruneResult{}, err
	}

	var report image.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return ImagePruneResult{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return ImagePruneResult{Report: report}, nil
}
