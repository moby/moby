package checkpoint

import (
	"github.com/moby/moby/api/types/checkpoint"
	"github.com/moby/moby/v2/daemon/server/backend"
)

// Backend for Checkpoint
type Backend interface {
	CheckpointCreate(container string, config checkpoint.CreateRequest) error
	CheckpointDelete(container string, config backend.CheckpointDeleteOptions) error
	CheckpointList(container string, config backend.CheckpointListOptions) ([]checkpoint.Summary, error)
}
