package client // import "github.com/docker/docker/client"

import (
	"context"
)

// CheckpointCreateOptions holds parameters to create a checkpoint from a container
type CheckpointCreateOptions struct {
	CheckpointID  string
	CheckpointDir string
	Exit          bool
}

// CheckpointCreate creates a checkpoint from the given container with the given name
func (cli *Client) CheckpointCreate(ctx context.Context, container string, options CheckpointCreateOptions) error {
	resp, err := cli.post(ctx, "/containers/"+container+"/checkpoints", nil, options, nil)
	ensureReaderClosed(resp)
	return err
}
