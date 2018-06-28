package llb

import (
	"context"

	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func WithMetaResolver(mr ImageMetaResolver) ImageOption {
	return ImageOptionFunc(func(ii *ImageInfo) {
		ii.metaResolver = mr
	})
}

type ImageMetaResolver interface {
	ResolveImageConfig(ctx context.Context, ref string, platform *specs.Platform) (digest.Digest, []byte, error)
}
