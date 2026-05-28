package client

import (
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImagePullOptions holds information to pull images.
type ImagePullOptions struct {
	All          bool
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/moby/moby/api/types/registry.RequestAuthConfig].
	PrivilegeFunc func(context.Context) (string, error)

	// Platforms selects the platforms to pull. Multiple platforms can be
	// specified if the image ia a multi-platform image.
	Platforms []ocispec.Platform
}
