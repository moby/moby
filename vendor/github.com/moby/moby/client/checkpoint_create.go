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

// CheckpointCreateResult holds the result from [client.CheckpointCreate].
type CheckpointCreateResult struct {
	// Add future fields here
}

// CheckpointCreate creates a checkpoint from the given container.
func (cli *Client) CheckpointCreate(ctx context.Context, containerID string, options CheckpointCreateOptions) (CheckpointCreateResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return CheckpointCreateResult{}, err
	}
	requestBody := checkpoint.CreateRequest{
		CheckpointID:  options.CheckpointID,
		CheckpointDir: options.CheckpointDir,
		Exit:          options.Exit,
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/checkpoints", nil, requestBody, nil)
	defer ensureReaderClosed(resp)
	return CheckpointCreateResult{}, err
}
