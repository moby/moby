package client

import (
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageCreateOptions holds information to create images.
type ImageCreateOptions struct {
	RegistryAuth string // RegistryAuth is the base64 encoded credentials for the registry.
	// Platforms specifies the platforms to platform of the image if it needs
	// to be pulled from the registry. Multiple platforms can be provided
	// if the daemon supports multi-platform pulls.
	Platforms []ocispec.Platform
}

// ImageCreateResult holds the response body returned by the daemon for image create.
type ImageCreateResult struct {
	Body io.ReadCloser
}
