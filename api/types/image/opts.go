package image

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/filters"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImportSource holds source information for ImageImport
type ImportSource struct {
	Source     io.Reader // Source is the data to send to the server to create this image from. You must set SourceName to "-" to leverage this.
	SourceName string    // SourceName is the name of the image to pull. Set to "-" to leverage the Source attribute.
}

// ImportOptions holds information to import images from the client host.
type ImportOptions struct {
	Tag      string   // Tag is the name to tag this image with. This attribute is deprecated.
	Message  string   // Message is the message to tag the image with
	Changes  []string // Changes are the raw changes to apply to this image
	Platform string   // Platform is the target platform of the image
}

// CreateOptions holds information to create images.
type CreateOptions struct {
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry.
	Platform     string // Platform is the target platform of the image if it needs to be pulled from the registry.
}

// PullOptions holds information to pull images.
type PullOptions struct {
	All          bool
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/docker/docker/api/types/registry.RequestAuthConfig].
	PrivilegeFunc func(context.Context) (string, error)
	Platform      string

	// ChallengeHandlerFunc is a function that a client can supply to handle
	// challenges returned by the registry.
	// If ChallengeHandlerFunc is not nil, the engine will not attempt to handle
	// any challenges returned by the registry. Instead, the engine will return
	// a 401 response to the client, including the WWW-Authenticate header, and
	// the client will call ChallengeHandlerFunc to handle the challenge.
	// This function returns the registry authentication header value (in base64
	// encoded format).
	ChallengeHandlerFunc func(context.Context, string) (string, error)
}

// PushOptions holds information to push images.
type PushOptions struct {
	All          bool
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/docker/docker/api/types/registry.RequestAuthConfig].
	PrivilegeFunc func(context.Context) (string, error)

	// Platform is an optional field that selects a specific platform to push
	// when the image is a multi-platform image.
	// Using this will only push a single platform-specific manifest.
	Platform *ocispec.Platform `json:",omitempty"`

	// ChallengeHandlerFunc is a function that a client can supply to handle
	// challenges returned by the registry.
	// If ChallengeHandlerFunc is not nil, the engine will not attempt to handle
	// any challenges returned by the registry. Instead, the engine will return
	// a 401 response to the client, including the WWW-Authenticate header, and
	// the client will call ChallengeHandlerFunc to handle the challenge.
	// This function returns the registry authentication header value (in base64
	// encoded format).
	ChallengeHandlerFunc func(context.Context, string) (string, error)
}

// ListOptions holds parameters to list images with.
type ListOptions struct {
	// All controls whether all images in the graph are filtered, or just
	// the heads.
	All bool

	// Filters is a JSON-encoded set of filter arguments.
	Filters filters.Args

	// SharedSize indicates whether the shared size of images should be computed.
	SharedSize bool

	// ContainerCount indicates whether container count should be computed.
	ContainerCount bool

	// Manifests indicates whether the image manifests should be returned.
	Manifests bool
}

// RemoveOptions holds parameters to remove images.
type RemoveOptions struct {
	Force         bool
	PruneChildren bool
}

// HistoryOptions holds parameters to get image history.
type HistoryOptions struct {
	// Platform from the manifest list to use for history.
	Platform *ocispec.Platform
}

// LoadOptions holds parameters to load images.
type LoadOptions struct {
	// Quiet suppresses progress output
	Quiet bool

	// Platforms selects the platforms to load if the image is a
	// multi-platform image and has multiple variants.
	Platforms []ocispec.Platform
}

// SaveOptions holds parameters to save images.
type SaveOptions struct {
	// Platforms selects the platforms to save if the image is a
	// multi-platform image and has multiple variants.
	Platforms []ocispec.Platform
}
