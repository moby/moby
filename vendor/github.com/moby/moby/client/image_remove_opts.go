package client

import (
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageRemoveOptions holds parameters to remove images.
type ImageRemoveOptions struct {
	Platforms     []ocispec.Platform
	Force         bool
	PruneChildren bool
}

// ImageRemoveResult holds the delete responses returned by the daemon.
type ImageRemoveResult struct {
	Items []image.DeleteResponse
}
