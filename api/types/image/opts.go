package image

import specs "github.com/opencontainers/image-spec/specs-go/v1"

// GetImageOpts holds parameters to inspect an image.
type GetImageOpts struct {
	Platform *specs.Platform
}
