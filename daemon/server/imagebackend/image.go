package imagebackend

import (
	"io"
	"net/http"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type PullOptions struct {
	Platforms   []ocispec.Platform
	MetaHeaders http.Header
	AuthConfig  *registry.AuthConfig
	OutStream   io.Writer
}

type PushOptions struct {
	Platforms   []ocispec.Platform
	MetaHeaders http.Header
	AuthConfig  *registry.AuthConfig
	OutStream   io.Writer
}

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

// GetImageOpts holds parameters to retrieve image information
// from the backend.
type GetImageOpts struct {
	Platform *ocispec.Platform
}

// ImageInspectOpts holds parameters to inspect an image.
type ImageInspectOpts struct {
	Manifests bool
	Platform  *ocispec.Platform
}
