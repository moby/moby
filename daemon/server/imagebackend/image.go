package imagebackend

import (
	"github.com/moby/moby/api/types/filters"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type RemoveOptions struct {
	Platforms     []ocispec.Platform
	Force         bool
	PruneChildren bool
}

type ListOptions struct {
	// All controls whether all images in the graph are filtered, or just
	// the heads.
	All bool

	// Filters is a JSON-encoded set of filter arguments.
	Filters filters.Args

	// SharedSize indicates whether the shared size of images should be computed.
	SharedSize bool

	// Manifests indicates whether the image manifests should be returned.
	Manifests bool
}
