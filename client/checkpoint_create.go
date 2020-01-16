package client // import "github.com/moby/moby/client"

import (
	"context"

	"github.com/moby/moby/api/types"
)

// CheckpointCreate creates a checkpoint from the given container with the given name
func (cli *Client) CheckpointCreate(ctx context.Context, container string, options types.CheckpointCreateOptions) error {
	resp, err := cli.post(ctx, "/containers/"+container+"/checkpoints", nil, options, nil)
	ensureReaderClosed(resp)
	return err
}
