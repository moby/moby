package checkpoint // import "github.com/docker/docker/api/server/router/checkpoint"

import (
	"context"

	"github.com/docker/docker/api/types"
)

// Backend for Checkpoint
type Backend interface {
	CheckpointCreate(ctx context.Context, container string, config types.CheckpointCreateOptions) error
	CheckpointDelete(ctx context.Context, container string, config types.CheckpointDeleteOptions) error
	CheckpointList(ctx context.Context, container string, config types.CheckpointListOptions) ([]types.Checkpoint, error)
}
