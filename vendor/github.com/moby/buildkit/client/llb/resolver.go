package llb

import (
	"context"

	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
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
type ImageMetaResolver interface {
	ResolveImageConfig(ctx context.Context, ref string, opt ResolveImageConfigOpt) (digest.Digest, []byte, error)
}

type ResolverType int

const (
	ResolverTypeRegistry ResolverType = iota
	ResolverTypeOCILayout
)

type ResolveImageConfigOpt struct {
	ResolverType

	Platform    *ocispecs.Platform
	ResolveMode string
	LogName     string

	Store ResolveImageConfigOptStore
}

type ResolveImageConfigOptStore struct {
	SessionID string
	StoreID   string
}
