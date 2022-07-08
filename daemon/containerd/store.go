package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// fakeStoreWithSources fakes the existence of the specified content.
// Only existence is faked - Info function will include the distribution source label
// which makes it possible to perform cross-repo mount.
// ReaderAt will still fail with ErrNotFound.
type fakeStoreWithSources struct {
	s       content.Store
	sources map[digest.Digest]distributionSource
}

// wrapWithFakeMountableBlobs wraps the provided content store.
func wrapWithFakeMountableBlobs(s content.Store, sources map[digest.Digest]distributionSource) fakeStoreWithSources {
	return fakeStoreWithSources{
		s:       s,
		sources: sources,
	}
}

func (p fakeStoreWithSources) Delete(ctx context.Context, dgst digest.Digest) error {
	return p.s.Delete(ctx, dgst)
}

func (p fakeStoreWithSources) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	info, err := p.s.Info(ctx, dgst)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			return info, err
		}
		source, ok := p.sources[dgst]
		if !ok {
			return info, err
		}

		key := labelDistributionSource + reference.Domain(source.registryRef)
		value := reference.Path(source.registryRef)
		return content.Info{
			Digest: dgst,
			Labels: map[string]string{
				key: value,
			},
		}, nil
	}

	return info, nil
}

func (p fakeStoreWithSources) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return p.s.Update(ctx, info, fieldpaths...)
}

func (p fakeStoreWithSources) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return p.s.Walk(ctx, fn, filters...)
}

func (p fakeStoreWithSources) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	return p.s.ReaderAt(ctx, desc)
}

func (p fakeStoreWithSources) Abort(ctx context.Context, ref string) error {
	return p.s.Abort(ctx, ref)
}

func (p fakeStoreWithSources) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	return p.s.ListStatuses(ctx, filters...)
}

func (p fakeStoreWithSources) Status(ctx context.Context, ref string) (content.Status, error) {
	return p.s.Status(ctx, ref)
}

func (p fakeStoreWithSources) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return p.s.Writer(ctx, opts...)
}
