package client

import (
	"context"
	"net/url"
)

// CheckpointDeleteOptions holds parameters to delete a checkpoint from a container.
type CheckpointDeleteOptions struct {
	CheckpointID  string
	CheckpointDir string
}

// CheckpointDelete deletes the checkpoint with the given name from the given container.
func (cli *Client) CheckpointDelete(ctx context.Context, containerID string, options CheckpointDeleteOptions) error {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return err
	}

	query := url.Values{}
	if options.CheckpointDir != "" {
		query.Set("dir", options.CheckpointDir)
	}

	resp, err := cli.delete(ctx, "/containers/"+containerID+"/checkpoints/"+options.CheckpointID, query, nil)
	defer ensureReaderClosed(resp)
	return err
}
