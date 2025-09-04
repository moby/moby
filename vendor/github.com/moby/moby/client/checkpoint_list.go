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

// CheckpointList returns the checkpoints of the given container in the docker host.
func (cli *Client) CheckpointList(ctx context.Context, container string, options CheckpointListOptions) ([]checkpoint.Summary, error) {
	var checkpoints []checkpoint.Summary

	query := url.Values{}
	if options.CheckpointDir != "" {
		query.Set("dir", options.CheckpointDir)
	}

	resp, err := cli.get(ctx, "/containers/"+container+"/checkpoints", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return checkpoints, err
	}

	err = json.NewDecoder(resp.Body).Decode(&checkpoints)
	return checkpoints, err
}
