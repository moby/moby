// +build experimental

package types

// ContainerState stores container's running state
type ContainerState struct {
	ContainerStateBase
	Checkpointed   bool
	CheckpointedAt string
}
