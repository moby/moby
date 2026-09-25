package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/checkpoint"
)

// CheckpointListOptions holds parameters to list checkpoints for a container.
type CheckpointListOptions struct {
	CheckpointDir string
}

// CheckpointListResult holds the result from the CheckpointList method.
type CheckpointListResult struct {
	Items []checkpoint.Summary
}

// CheckpointList returns the checkpoints of the given container in the docker host.
func (cli *Client) CheckpointList(ctx context.Context, container string, options CheckpointListOptions) (CheckpointListResult, error) {
	var out CheckpointListResult

	query := url.Values{}
	if options.CheckpointDir != "" {
		query.Set("dir", options.CheckpointDir)
	}

	resp, err := cli.get(ctx, "/containers/"+container+"/checkpoints", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return out, err
	}

	err = json.NewDecoder(resp.Body).Decode(&out.Items)
	return out, err
}
