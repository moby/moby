package llb

import (
	"context"

	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// WithMetaResolver adds a metadata resolver to an image
func WithMetaResolver(mr ImageMetaResolver) ImageOption {
	return imageOptionFunc(func(ii *ImageInfo) {
		ii.metaResolver = mr
	})
}

// ImageMetaResolver can resolve image config metadata from a reference
type ImageMetaResolver interface {
	ResolveImageConfig(ctx context.Context, ref string, opt ResolveImageConfigOpt) (digest.Digest, []byte, error)
}

type ResolveImageConfigOpt struct {
	Platform    *specs.Platform
	ResolveMode string
	LogName     string
}
