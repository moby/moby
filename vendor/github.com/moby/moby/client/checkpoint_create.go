package client

import (
	"context"

	"github.com/moby/moby/api/types/checkpoint"
)

// CheckpointCreateOptions holds parameters to create a checkpoint from a container.
type CheckpointCreateOptions struct {
	CheckpointID  string
	CheckpointDir string
	Exit          bool
}

// CheckpointCreate creates a checkpoint from the given container.
func (cli *Client) CheckpointCreate(ctx context.Context, containerID string, options CheckpointCreateOptions) error {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return err
	}
	requestBody := checkpoint.CreateRequest{
		CheckpointID:  options.CheckpointID,
		CheckpointDir: options.CheckpointDir,
		Exit:          options.Exit,
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/checkpoints", nil, requestBody, nil)
	defer ensureReaderClosed(resp)
	return err
}
