package checkpoint

import (
	"github.com/moby/moby/api/types/checkpoint"
	"github.com/moby/moby/client"
)

// Summary represents the details of a checkpoint when listing endpoints.
type Summary = checkpoint.Summary

// CreateOptions holds parameters to create a checkpoint from a container.
type CreateOptions = client.CheckpointCreateOptions

// ListOptions holds parameters to list checkpoints for a container.
type ListOptions = client.CheckpointListOptions

// DeleteOptions holds parameters to delete a checkpoint from a container.
type DeleteOptions = client.CheckpointDeleteOptions
