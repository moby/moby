package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/container"
)

// ContainerPruneOptions holds parameters to prune containers.
type ContainerPruneOptions struct {
	Filters Filters
}

// ContainerPruneResult holds the result from the [Client.ContainersPrune] method.
type ContainerPruneResult struct {
	Report container.PruneReport
}

// ContainersPrune requests the daemon to delete unused data
func (cli *Client) ContainersPrune(ctx context.Context, opts ContainerPruneOptions) (ContainerPruneResult, error) {
	query := url.Values{}
	opts.Filters.updateURLValues(query)

	resp, err := cli.post(ctx, "/containers/prune", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerPruneResult{}, err
	}

	var report container.PruneReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return ContainerPruneResult{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return ContainerPruneResult{Report: report}, nil
}
