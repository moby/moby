package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func NewContentStore(store content.Store, ns string) content.Store {
	return &nsContent{ns, store}
}

type nsContent struct {
	ns string
	content.Store
}

func (c *nsContent) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Info(ctx, dgst)
}

func (c *nsContent) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Update(ctx, info, fieldpaths...)
}

func (c *nsContent) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Walk(ctx, fn, filters...)
}

func (c *nsContent) Delete(ctx context.Context, dgst digest.Digest) error {
	return errors.Errorf("contentstore.Delete usage is forbidden")
}

func (c *nsContent) Status(ctx context.Context, ref string) (content.Status, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Status(ctx, ref)
}

func (c *nsContent) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.ListStatuses(ctx, filters...)
}

func (c *nsContent) Abort(ctx context.Context, ref string) error {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Abort(ctx, ref)
}

func (c *nsContent) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.ReaderAt(ctx, desc)
}

func (c *nsContent) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return c.writer(ctx, 3, opts...)
}

func (c *nsContent) writer(ctx context.Context, retries int, opts ...content.WriterOpt) (content.Writer, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	w, err := c.Store.Writer(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &nsWriter{Writer: w, ns: c.ns}, nil
}

type nsWriter struct {
	content.Writer
	ns string
}

func (w *nsWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	ctx = namespaces.WithNamespace(ctx, w.ns)
	return w.Writer.Commit(ctx, size, expected, opts...)
}
