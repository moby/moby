package client // import "github.com/docker/docker/client"

import (
	"context"

	"github.com/docker/docker/api/types/container"
)

type apiClientExperimental interface {
	CheckpointAPIClient
}

// CheckpointAPIClient defines API client methods for the checkpoints
type CheckpointAPIClient interface {
	CheckpointCreate(ctx context.Context, container string, options CheckpointCreateOptions) error
	CheckpointDelete(ctx context.Context, container string, options CheckpointDeleteOptions) error
	CheckpointList(ctx context.Context, container string, options CheckpointListOptions) ([]container.Checkpoint, error)
}
