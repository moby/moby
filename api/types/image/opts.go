package image

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

// GetImageOpts holds parameters to retrieve image information
// from the backend.
type GetImageOpts struct {
	Platform *ocispec.Platform
	Details  bool
}

// InspectOptions contains options for inspecting images.
type InspectOptions struct {
	// Platform specifies the platform for which to show the inspect data
	// if multiple variants exist for the inspected image.
	//
	// If no platform is set, the platform's default platform is used or,
	// if not present, the first available variant.
	Platform string
}
