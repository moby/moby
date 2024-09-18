package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/namespaces"
	cerrdefs "github.com/containerd/errdefs"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func NewContentStore(store content.Store, ns string) *Store {
	return &Store{ns, store}
}

type Store struct {
	ns string
	content.Store
}

func (c *Store) Namespace() string {
	return c.ns
}

func (c *Store) WithNamespace(ns string) *Store {
	return NewContentStore(c.Store, ns)
}

func (c *Store) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Info(ctx, dgst)
}

func (c *Store) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Update(ctx, info, fieldpaths...)
}

func (c *Store) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Walk(ctx, fn, filters...)
}

func (c *Store) Delete(ctx context.Context, dgst digest.Digest) error {
	return errors.Errorf("contentstore.Delete usage is forbidden")
}

func (c *Store) Status(ctx context.Context, ref string) (content.Status, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Status(ctx, ref)
}

func (c *Store) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.ListStatuses(ctx, filters...)
}

func (c *Store) Abort(ctx context.Context, ref string) error {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.Abort(ctx, ref)
}

func (c *Store) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	return c.Store.ReaderAt(ctx, desc)
}

func (c *Store) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	ctx = namespaces.WithNamespace(ctx, c.ns)
	w, err := c.Store.Writer(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &nsWriter{Writer: w, ns: c.ns}, nil
}

func (c *Store) WithFallbackNS(ns string) content.Store {
	return &nsFallbackStore{
		main: c,
		fb:   c.WithNamespace(ns),
	}
}

type nsWriter struct {
	content.Writer
	ns string
}

func (w *nsWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	ctx = namespaces.WithNamespace(ctx, w.ns)
	return w.Writer.Commit(ctx, size, expected, opts...)
}

type nsFallbackStore struct {
	main *Store
	fb   *Store
}

var _ content.Store = &nsFallbackStore{}

func (c *nsFallbackStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	info, err := c.main.Info(ctx, dgst)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return c.fb.Info(ctx, dgst)
		}
	}
	return info, err
}

func (c *nsFallbackStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return c.main.Update(ctx, info, fieldpaths...)
}

func (c *nsFallbackStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return c.main.Walk(ctx, fn, filters...)
}

func (c *nsFallbackStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return c.main.Delete(ctx, dgst)
}

func (c *nsFallbackStore) Status(ctx context.Context, ref string) (content.Status, error) {
	return c.main.Status(ctx, ref)
}

func (c *nsFallbackStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	return c.main.ListStatuses(ctx, filters...)
}

func (c *nsFallbackStore) Abort(ctx context.Context, ref string) error {
	return c.main.Abort(ctx, ref)
}

func (c *nsFallbackStore) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	ra, err := c.main.ReaderAt(ctx, desc)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return c.fb.ReaderAt(ctx, desc)
		}
	}
	return ra, err
}

func (c *nsFallbackStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return c.main.Writer(ctx, opts...)
}
