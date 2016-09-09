// +build experimental

package checkpoint

import "github.com/docker/docker/api/types"

// Backend for Checkpoint
type Backend interface {
	CheckpointCreate(container string, config types.CheckpointCreateOptions) error
	CheckpointDelete(container string, checkpointID string) error
	CheckpointList(container string) ([]types.Checkpoint, error)
}
