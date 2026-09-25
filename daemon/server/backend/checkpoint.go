package backend

// CheckpointListOptions holds parameters to list checkpoints for a container.
type CheckpointListOptions struct {
	CheckpointDir string
}

// CheckpointDeleteOptions holds parameters to delete a checkpoint from a container.
type CheckpointDeleteOptions struct {
	CheckpointID  string
	CheckpointDir string
}
