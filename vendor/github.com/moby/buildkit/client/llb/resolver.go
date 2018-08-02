package llb

import (
	"context"

	gw "github.com/moby/buildkit/frontend/gateway/client"
	digest "github.com/opencontainers/go-digest"
)

func WithMetaResolver(mr ImageMetaResolver) ImageOption {
	return imageOptionFunc(func(ii *ImageInfo) {
		ii.metaResolver = mr
	})
}

type ImageMetaResolver interface {
	ResolveImageConfig(ctx context.Context, ref string, opt gw.ResolveImageConfigOpt) (digest.Digest, []byte, error)
}
