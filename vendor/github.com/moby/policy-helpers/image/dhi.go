package image

import (
	"context"
	"strings"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

type dhiKey struct{}

func isDHIIndex(idx ocispecs.Index) bool {
	for _, desc := range idx.Manifests {
		if buildid, ok := desc.Annotations["com.docker.dhi.build.id"]; !ok || buildid == "" {
			return false
		}
	}
	return strings.HasPrefix(idx.Annotations["org.opencontainers.image.title"], "dhi/")
}

func contextWithDHI(ctx context.Context) context.Context {
	return context.WithValue(ctx, dhiKey{}, struct{}{})
}

func IsDHI(ctx context.Context) bool {
	_, ok := ctx.Value(dhiKey{}).(struct{})
	return ok
}

type dhiReferrersProvider struct {
	ReferrersProvider
}

func (d *dhiReferrersProvider) FetchReferrers(ctx context.Context, dgst digest.Digest, opts ...remotes.FetchReferrersOpt) ([]ocispecs.Descriptor, error) {
	ctx = contextWithDHI(ctx)
	return d.ReferrersProvider.FetchReferrers(ctx, dgst, opts...)
}

func (d *dhiReferrersProvider) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	ctx = contextWithDHI(ctx)
	return d.ReferrersProvider.ReaderAt(ctx, desc)
}
