package checkpoint

import "github.com/moby/moby/api/types/checkpoint"

// Summary represents the details of a checkpoint when listing endpoints.
type Summary = checkpoint.Summary

// CreateOptions holds parameters to create a checkpoint from a container.
type CreateOptions = checkpoint.CreateOptions

// ListOptions holds parameters to list checkpoints for a container.
type ListOptions = checkpoint.ListOptions

// DeleteOptions holds parameters to delete a checkpoint from a container.
type DeleteOptions = checkpoint.DeleteOptions
