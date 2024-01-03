package containerimage

import (
	"context"
	"io"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	sessioncontent "github.com/moby/buildkit/session/content"
	"github.com/moby/buildkit/util/imageutil"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	maxReadSize = 4 * 1024 * 1024
)

// getOCILayoutResolver gets a resolver to an OCI layout for a specified store from the client using the given session.
func getOCILayoutResolver(store llb.ResolveImageConfigOptStore, sm *session.Manager, g session.Group) *ociLayoutResolver {
	r := &ociLayoutResolver{
		store: store,
		sm:    sm,
		g:     g,
	}
	return r
}

type ociLayoutResolver struct {
	remotes.Resolver
	store llb.ResolveImageConfigOptStore
	sm    *session.Manager
	g     session.Group
}

// Fetcher returns a new fetcher for the provided reference.
func (r *ociLayoutResolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	return r, nil
}

// Fetch get an io.ReadCloser for the specific content
func (r *ociLayoutResolver) Fetch(ctx context.Context, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := r.withCaller(ctx, func(ctx context.Context, caller session.Caller) error {
		store := sessioncontent.NewCallerStore(caller, "oci:"+r.store.StoreID)
		readerAt, err := store.ReaderAt(ctx, desc)
		if err != nil {
			return err
		}
		rc = &readerAtWrapper{readerAt: readerAt}
		return nil
	})
	return rc, err
}

// Resolve attempts to resolve the reference into a name and descriptor.
// OCI Layout does not (yet) support tag name references, but does support hash references.
func (r *ociLayoutResolver) Resolve(ctx context.Context, refString string) (string, ocispecs.Descriptor, error) {
	ref, err := reference.Parse(refString)
	if err != nil {
		return "", ocispecs.Descriptor{}, errors.Wrapf(err, "invalid reference %q", refString)
	}
	dgst := ref.Digest()
	if dgst == "" {
		return "", ocispecs.Descriptor{}, errors.Errorf("reference %q must have digest", refString)
	}

	info, err := r.info(ctx, ref)
	if err != nil {
		return "", ocispecs.Descriptor{}, errors.Wrap(err, "unable to get info about digest")
	}

	// Create the descriptor, then use that to read the actual root manifest/
	// This is necessary because we do not know the media-type of the descriptor,
	// and there are descriptor processing elements that expect it.
	desc := ocispecs.Descriptor{
		Digest: info.Digest,
		Size:   info.Size,
	}
	rc, err := r.Fetch(ctx, desc)
	if err != nil {
		return "", ocispecs.Descriptor{}, errors.Wrap(err, "unable to get root manifest")
	}
	b, err := io.ReadAll(io.LimitReader(rc, maxReadSize))
	if err != nil {
		return "", ocispecs.Descriptor{}, errors.Wrap(err, "unable to read root manifest")
	}

	mediaType, err := imageutil.DetectManifestBlobMediaType(b)
	if err != nil {
		return "", ocispecs.Descriptor{}, errors.Wrapf(err, "reference %q contains neither an index nor a manifest", refString)
	}
	desc.MediaType = mediaType

	return refString, desc, nil
}

func (r *ociLayoutResolver) info(ctx context.Context, ref reference.Spec) (content.Info, error) {
	var info *content.Info
	err := r.withCaller(ctx, func(ctx context.Context, caller session.Caller) error {
		store := sessioncontent.NewCallerStore(caller, "oci:"+r.store.StoreID)

		_, dgst := reference.SplitObject(ref.Object)
		if dgst == "" {
			return errors.Errorf("reference %q does not contain a digest", ref.String())
		}
		in, err := store.Info(ctx, dgst)
		info = &in
		return err
	})
	if err != nil {
		return content.Info{}, err
	}
	if info == nil {
		return content.Info{}, errors.Errorf("reference %q did not match any content", ref.String())
	}
	return *info, nil
}

func (r *ociLayoutResolver) withCaller(ctx context.Context, f func(context.Context, session.Caller) error) error {
	if r.store.SessionID != "" {
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		caller, err := r.sm.Get(timeoutCtx, r.store.SessionID, false)
		if err != nil {
			return err
		}
		return f(ctx, caller)
	}
	return r.sm.Any(ctx, r.g, func(ctx context.Context, _ string, caller session.Caller) error {
		return f(ctx, caller)
	})
}

// readerAtWrapper wraps a ReaderAt to give a Reader
type readerAtWrapper struct {
	offset   int64
	readerAt content.ReaderAt
}

func (r *readerAtWrapper) Read(p []byte) (n int, err error) {
	n, err = r.readerAt.ReadAt(p, r.offset)
	r.offset += int64(n)
	return
}
func (r *readerAtWrapper) Close() error {
	return r.readerAt.Close()
}
