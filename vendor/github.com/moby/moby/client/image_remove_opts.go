package client

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

// ImageRemoveOptions holds parameters to remove images.
type ImageRemoveOptions struct {
	Platforms     []ocispec.Platform
	Force         bool
	PruneChildren bool
}
