package checkpoint

// CreateRequest holds parameters to create a checkpoint from a container.
type CreateRequest struct {
	CheckpointID  string
	CheckpointDir string
	Exit          bool
}
