package client

import (
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImagePushOptions holds information to push images.
type ImagePushOptions struct {
	All          bool
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/moby/moby/api/types/registry.RequestAuthConfig].
	PrivilegeFunc func(context.Context) (string, error)

	// ChallengeHandlerFunc is a function that a client can supply to handle
	// challenges returned by the registry.
	// If ChallengeHandlerFunc is not nil, the engine will not attempt to handle
	// any challenges returned by the registry. Instead, the engine will return
	// a 401 response to the client, including the WWW-Authenticate header, and
	// the client will call ChallengeHandlerFunc to handle the challenge.
	// This function returns the registry authentication header value (in base64
	// encoded format).
	ChallengeHandlerFunc func(context.Context, string) (string, error)

	// Platform is an optional field that selects a specific platform to push
	// when the image is a multi-platform image.
	// Using this will only push a single platform-specific manifest.
	Platform *ocispec.Platform `json:",omitempty"`
}
