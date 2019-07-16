package content

import (
	"context"

	api "github.com/containerd/containerd/api/services/content/v1"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/proxy"
	"github.com/moby/buildkit/session"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"google.golang.org/grpc/metadata"
)

type callerContentStore struct {
	store   content.Store
	storeID string
}

func (cs *callerContentStore) choose(ctx context.Context) context.Context {
	nsheader := metadata.Pairs(GRPCHeaderID, cs.storeID)
	md, ok := metadata.FromOutgoingContext(ctx) // merge with outgoing context.
	if !ok {
		md = nsheader
	} else {
		// order ensures the latest is first in this list.
		md = metadata.Join(nsheader, md)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

func (cs *callerContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	ctx = cs.choose(ctx)
	info, err := cs.store.Info(ctx, dgst)
	return info, errors.WithStack(err)
}

func (cs *callerContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	ctx = cs.choose(ctx)
	info, err := cs.store.Update(ctx, info, fieldpaths...)
	return info, errors.WithStack(err)
}

func (cs *callerContentStore) Walk(ctx context.Context, fn content.WalkFunc, fs ...string) error {
	ctx = cs.choose(ctx)
	return errors.WithStack(cs.store.Walk(ctx, fn, fs...))
}

func (cs *callerContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	ctx = cs.choose(ctx)
	return errors.WithStack(cs.store.Delete(ctx, dgst))
}

func (cs *callerContentStore) ListStatuses(ctx context.Context, fs ...string) ([]content.Status, error) {
	ctx = cs.choose(ctx)
	resp, err := cs.store.ListStatuses(ctx, fs...)
	return resp, errors.WithStack(err)
}

func (cs *callerContentStore) Status(ctx context.Context, ref string) (content.Status, error) {
	ctx = cs.choose(ctx)
	st, err := cs.store.Status(ctx, ref)
	return st, errors.WithStack(err)
}

func (cs *callerContentStore) Abort(ctx context.Context, ref string) error {
	ctx = cs.choose(ctx)
	return errors.WithStack(cs.store.Abort(ctx, ref))
}

func (cs *callerContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	ctx = cs.choose(ctx)
	w, err := cs.store.Writer(ctx, opts...)
	return w, errors.WithStack(err)
}

func (cs *callerContentStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	ctx = cs.choose(ctx)
	ra, err := cs.store.ReaderAt(ctx, desc)
	return ra, errors.WithStack(err)
}

// NewCallerStore creates content.Store from session.Caller with specified storeID
func NewCallerStore(c session.Caller, storeID string) content.Store {
	client := api.NewContentClient(c.Conn())
	return &callerContentStore{
		store:   proxy.NewContentStore(client),
		storeID: storeID,
	}
}
