package imagebackend

import (
	"io"
	"net/http"

	"github.com/moby/moby/api/types/container"
	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/storage"
	"github.com/moby/moby/v2/daemon/internal/filters"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type PullOptions struct {
	Platforms   []ocispec.Platform
	MetaHeaders http.Header
	AuthConfig  *registry.AuthConfig
	OutStream   io.Writer
}

type PushOptions struct {
	Platforms            []ocispec.Platform
	ForceCheckLayerExist bool
	MetaHeaders          http.Header
	AuthConfig           *registry.AuthConfig
	OutStream            io.Writer
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

type InspectData struct {
	imagetypes.InspectResponse

	// Parent is the ID of the parent image.
	//
	// Depending on how the image was created, this field may be empty and
	// is only set for images that were built/created locally. This field
	// is omitted if the image was pulled from an image registry.
	//
	// This field is deprecated with the legacy builder, but returned by the API if present.
	Parent string `json:",omitempty"`

	// DockerVersion is the version of Docker that was used to build the image.
	//
	// Depending on how the image was created, this field may be omitted.
	//
	// This field is deprecated with the legacy builder, but returned by the API if present.
	DockerVersion string `json:",omitempty"`

	// Container is the ID of the container that was used to create the image.
	//
	// Depending on how the image was created, this field may be empty.
	//
	// This field is removed in API v1.45, but used for API <= v1.44 responses.
	Container string

	// ContainerConfig is an optional field containing the configuration of the
	// container that was last committed when creating the image.
	//
	// Previous versions of Docker builder used this field to store build cache,
	// and it is not in active use anymore.
	//
	// This field is removed in API v1.45, but used for API <= v1.44 responses.
	ContainerConfig *container.Config

	// GraphDriverLegacy is used for API versions < v1.52, which included the
	// name of the snapshotter the GraphDriver field.
	GraphDriverLegacy *storage.DriverData
}
