package client

import (
	"context"
)

// CheckpointAPIClient defines API client methods for the checkpoints.
//
// Experimental: checkpoint and restore is still an experimental feature,
// and only available if the daemon is running with experimental features
// enabled.
type CheckpointAPIClient interface {
	CheckpointCreate(ctx context.Context, container string, options CheckpointCreateOptions) error
	CheckpointDelete(ctx context.Context, container string, options CheckpointDeleteOptions) error
	CheckpointList(ctx context.Context, container string, options CheckpointListOptions) (CheckpointListResult, error)
}
