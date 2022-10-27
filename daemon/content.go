package daemon

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

func (daemon *Daemon) configureLocalContentStore(ns string) (content.Store, leases.Manager, error) {
	if err := os.MkdirAll(filepath.Join(daemon.root, "content"), 0700); err != nil {
		return nil, nil, errors.Wrap(err, "error creating dir for content store")
	}
	db, err := bbolt.Open(filepath.Join(daemon.root, "content", "metadata.db"), 0600, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error opening bolt db for content metadata store")
	}
	cs, err := local.NewStore(filepath.Join(daemon.root, "content", "data"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "error setting up content store")
	}
	md := metadata.NewDB(db, cs, nil)
	daemon.mdDB = db
	return namespacedContentProvider(md.ContentStore(), ns), namespacedLeaseManager(metadata.NewLeaseManager(md), ns), nil
}

// withDefaultNamespace sets the given namespace on the context if the current
// context doesn't hold any namespace
func withDefaultNamespace(ctx context.Context, namespace string) context.Context {
	if _, ok := namespaces.Namespace(ctx); ok {
		return ctx
	}
	return namespaces.WithNamespace(ctx, namespace)
}

type namespacedContent struct {
	ns       string
	provider content.Store
}

// Delete removes the content from the store.
func (cp namespacedContent) Delete(ctx context.Context, dgst digest.Digest) error {
	return cp.provider.Delete(withDefaultNamespace(ctx, cp.ns), dgst)
}

// Info will return metadata about content available in the content store.
//
// If the content is not present, ErrNotFound will be returned.
func (cp namespacedContent) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	return cp.provider.Info(withDefaultNamespace(ctx, cp.ns), dgst)
}

// Update updates mutable information related to content.
// If one or more fieldpaths are provided, only those
// fields will be updated.
// Mutable fields:
//
//	labels.*
func (cp namespacedContent) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return cp.provider.Update(withDefaultNamespace(ctx, cp.ns), info, fieldpaths...)
}

// Walk will call fn for each item in the content store which
// match the provided filters. If no filters are given all
// items will be walked.
func (cp namespacedContent) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return cp.provider.Walk(withDefaultNamespace(ctx, cp.ns), fn, filters...)
}

// Abort completely cancels the ingest operation targeted by ref.
func (cp namespacedContent) Abort(ctx context.Context, ref string) error {
	return cp.provider.Abort(withDefaultNamespace(ctx, cp.ns), ref)
}

// ListStatuses returns the status of any active ingestions whose ref match the
// provided regular expression. If empty, all active ingestions will be
// returned.
func (cp namespacedContent) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	return cp.provider.ListStatuses(withDefaultNamespace(ctx, cp.ns), filters...)
}

// Status returns the status of the provided ref.
func (cp namespacedContent) Status(ctx context.Context, ref string) (content.Status, error) {
	return cp.provider.Status(withDefaultNamespace(ctx, cp.ns), ref)
}

// Some implementations require WithRef to be included in opts.
func (cp namespacedContent) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return cp.provider.Writer(withDefaultNamespace(ctx, cp.ns), opts...)
}

// ReaderAt only requires desc.Digest to be set.
// Other fields in the descriptor may be used internally for resolving
// the location of the actual data.
func (cp namespacedContent) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	return cp.provider.ReaderAt(withDefaultNamespace(ctx, cp.ns), desc)
}

// namespacedContentProvider sets the namespace if missing before calling the inner provider
func namespacedContentProvider(provider content.Store, ns string) content.Store {
	return namespacedContent{
		ns,
		provider,
	}
}

type namespacedLeases struct {
	ns      string
	manager leases.Manager
}

// AddResource references the resource by the provided lease.
func (nl namespacedLeases) AddResource(ctx context.Context, lease leases.Lease, resource leases.Resource) error {
	return nl.manager.AddResource(withDefaultNamespace(ctx, nl.ns), lease, resource)
}

// Create creates a new lease using the provided lease
func (nl namespacedLeases) Create(ctx context.Context, opt ...leases.Opt) (leases.Lease, error) {
	return nl.manager.Create(withDefaultNamespace(ctx, nl.ns), opt...)
}

// Delete deletes the lease with the provided lease ID
func (nl namespacedLeases) Delete(ctx context.Context, lease leases.Lease, opt ...leases.DeleteOpt) error {
	return nl.manager.Delete(withDefaultNamespace(ctx, nl.ns), lease, opt...)
}

// DeleteResource dereferences the resource by the provided lease.
func (nl namespacedLeases) DeleteResource(ctx context.Context, lease leases.Lease, resource leases.Resource) error {
	return nl.manager.DeleteResource(withDefaultNamespace(ctx, nl.ns), lease, resource)
}

// List lists all active leases
func (nl namespacedLeases) List(ctx context.Context, filter ...string) ([]leases.Lease, error) {
	return nl.manager.List(withDefaultNamespace(ctx, nl.ns), filter...)
}

// ListResources lists all the resources referenced by the lease.
func (nl namespacedLeases) ListResources(ctx context.Context, lease leases.Lease) ([]leases.Resource, error) {
	return nl.manager.ListResources(withDefaultNamespace(ctx, nl.ns), lease)
}

// namespacedLeaseManager sets the namespace if missing before calling the inner manager
func namespacedLeaseManager(manager leases.Manager, ns string) leases.Manager {
	return namespacedLeases{
		ns,
		manager,
	}
}
