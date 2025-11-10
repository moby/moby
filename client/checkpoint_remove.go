package client

import (
	"context"
	"net/url"
)

// CheckpointRemoveOptions holds parameters to delete a checkpoint from a container.
type CheckpointRemoveOptions struct {
	CheckpointID  string
	CheckpointDir string
}

// CheckpointRemoveResult represents the result of [Client.CheckpointRemove].
type CheckpointRemoveResult struct {
	// No fields currently; placeholder for future use.
}

// CheckpointRemove deletes the checkpoint with the given name from the given container.
func (cli *Client) CheckpointRemove(ctx context.Context, containerID string, options CheckpointRemoveOptions) (CheckpointRemoveResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return CheckpointRemoveResult{}, err
	}

	query := url.Values{}
	if options.CheckpointDir != "" {
		query.Set("dir", options.CheckpointDir)
	}

	resp, err := cli.delete(ctx, "/containers/"+containerID+"/checkpoints/"+options.CheckpointID, query, nil)
	defer ensureReaderClosed(resp)
	return CheckpointRemoveResult{}, err
}
