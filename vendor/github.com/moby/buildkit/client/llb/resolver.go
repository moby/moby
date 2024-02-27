package llb

import (
	"github.com/moby/buildkit/client/llb/sourceresolver"
)

// WithMetaResolver adds a metadata resolver to an image
func WithMetaResolver(mr ImageMetaResolver) ImageOption {
	return imageOptionFunc(func(ii *ImageInfo) {
		ii.metaResolver = mr
	})
}

// ResolveDigest uses the meta resolver to update the ref of image with full digest before marshaling.
// This makes image ref immutable and is recommended if you want to make sure meta resolver data
// matches the image used during the build.
func ResolveDigest(v bool) ImageOption {
	return imageOptionFunc(func(ii *ImageInfo) {
		ii.resolveDigest = v
	})
}

func WithLayerLimit(l int) ImageOption {
	return imageOptionFunc(func(ii *ImageInfo) {
		ii.layerLimit = &l
	})
}

// ImageMetaResolver can resolve image config metadata from a reference
type ImageMetaResolver = sourceresolver.ImageMetaResolver
