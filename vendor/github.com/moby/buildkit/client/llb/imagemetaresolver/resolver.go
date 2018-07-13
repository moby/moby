package imagemetaresolver

import (
	"context"
	"net/http"
	"sync"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/docker/pkg/locker"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/imageutil"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

var defaultImageMetaResolver llb.ImageMetaResolver
var defaultImageMetaResolverOnce sync.Once

var WithDefault = llb.ImageOptionFunc(func(ii *llb.ImageInfo) {
	llb.WithMetaResolver(Default()).SetImageOption(ii)
})

type imageMetaResolverOpts struct {
	platform *specs.Platform
}

type ImageMetaResolverOpt func(o *imageMetaResolverOpts)

func WithDefaultPlatform(p *specs.Platform) ImageMetaResolverOpt {
	return func(o *imageMetaResolverOpts) {
		o.platform = p
	}
}

func New(with ...ImageMetaResolverOpt) llb.ImageMetaResolver {
	var opts imageMetaResolverOpts
	for _, f := range with {
		f(&opts)
	}
	return &imageMetaResolver{
		resolver: docker.NewResolver(docker.ResolverOptions{
			Client: http.DefaultClient,
		}),
		platform: opts.platform,
		buffer:   contentutil.NewBuffer(),
		cache:    map[string]resolveResult{},
		locker:   locker.New(),
	}
}

func Default() llb.ImageMetaResolver {
	defaultImageMetaResolverOnce.Do(func() {
		defaultImageMetaResolver = New()
	})
	return defaultImageMetaResolver
}

type imageMetaResolver struct {
	resolver remotes.Resolver
	buffer   contentutil.Buffer
	platform *specs.Platform
	locker   *locker.Locker
	cache    map[string]resolveResult
}

type resolveResult struct {
	config []byte
	dgst   digest.Digest
}

func (imr *imageMetaResolver) ResolveImageConfig(ctx context.Context, ref string, platform *specs.Platform) (digest.Digest, []byte, error) {
	imr.locker.Lock(ref)
	defer imr.locker.Unlock(ref)

	if res, ok := imr.cache[ref]; ok {
		return res.dgst, res.config, nil
	}

	if platform == nil {
		platform = imr.platform
	}

	dgst, config, err := imageutil.Config(ctx, ref, imr.resolver, imr.buffer, platform)
	if err != nil {
		return "", nil, err
	}

	imr.cache[ref] = resolveResult{dgst: dgst, config: config}
	return dgst, config, nil
}
