package client

import (
	"context"

	"github.com/moby/moby/api/types/checkpoint"
)

// CheckpointCreate creates a checkpoint from the given container.
func (cli *Client) CheckpointCreate(ctx context.Context, containerID string, options checkpoint.CreateOptions) error {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return err
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/checkpoints", nil, options, nil)
	defer ensureReaderClosed(resp)
	return err
}
