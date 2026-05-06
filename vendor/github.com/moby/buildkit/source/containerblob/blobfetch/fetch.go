// Package blobfetch provides a shared helper for resolving a single
// content-addressable blob from either a docker-image registry or an OCI layout
// content store, identified by its digest. It is factored out of the
// containerblob source so other consumers (for example the git-bundle flow on
// the git source) can reuse the same resolution logic without depending on the
// source implementation.
package blobfetch

import (
	"context"
	"io"
	"time"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/moby/buildkit/session"
	sessioncontent "github.com/moby/buildkit/session/content"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/iohelper"
	"github.com/moby/buildkit/util/resolver"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// FetchOpt describes how to resolve a blob.
type FetchOpt struct {
	// Scheme identifies the source kind. Must be one of
	// srctypes.DockerImageBlobScheme or srctypes.OCIBlobScheme.
	Scheme string
	// Ref is the image-ref form of the blob, i.e. "<name>@<digest>" for
	// registry blobs, or the reference body accepted by the containerd
	// reference parser for OCI-layout blobs.
	Ref string
	// Digest is the blob digest to retrieve.
	Digest digest.Digest
	// RegistryHosts is used when Scheme is DockerImageBlobScheme.
	RegistryHosts docker.RegistryHosts
	// SessionManager is used to reach the client when Scheme is
	// OCIBlobScheme.
	SessionManager *session.Manager
	// SessionID, if set, pins the OCI-layout fetch to a specific client
	// session.
	SessionID string
	// StoreID is the client-side OCI-layout store name. Required for
	// Scheme == OCIBlobScheme.
	StoreID string
}

// FetchBlob opens a read stream for the requested blob. The caller owns the
// returned ReadCloser and must Close it. The returned digest is the blob
// digest from the locator (echoed for convenience).
func FetchBlob(ctx context.Context, g session.Group, opt FetchOpt) (io.ReadCloser, digest.Digest, error) {
	if err := opt.Digest.Validate(); err != nil {
		return nil, "", errors.Wrap(err, "invalid blob digest")
	}
	switch opt.Scheme {
	case srctypes.OCIBlobScheme:
		if opt.StoreID == "" {
			return nil, "", errors.Errorf("oci-layout blob source requires store id")
		}
		rc, err := fetchFromOCILayoutStore(ctx, g, opt)
		if err != nil {
			return nil, "", err
		}
		return rc, opt.Digest, nil
	case srctypes.DockerImageBlobScheme:
		r := resolver.DefaultPool.GetResolver(opt.RegistryHosts, opt.Ref, resolver.ScopeType{}, opt.SessionManager, g)
		f, err := r.Fetcher(ctx, opt.Ref)
		if err != nil {
			return nil, "", err
		}
		fd, ok := f.(remotes.FetcherByDigest)
		if !ok {
			return nil, "", errors.Errorf("invalid blob fetcher: %T", f)
		}
		rc, _, err := fd.FetchByDigest(ctx, opt.Digest)
		if err != nil {
			return nil, "", err
		}
		return rc, opt.Digest, nil
	default:
		return nil, "", errors.Errorf("unsupported blob scheme %q", opt.Scheme)
	}
}

func fetchFromOCILayoutStore(ctx context.Context, g session.Group, opt FetchOpt) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := withOCICaller(ctx, g, opt, func(ctx context.Context, caller session.Caller) error {
		store := sessioncontent.NewCallerStore(caller, "oci:"+opt.StoreID)
		info, err := store.Info(ctx, opt.Digest)
		if err != nil {
			return err
		}

		readerAt, err := store.ReaderAt(ctx, ocispecs.Descriptor{
			Digest: info.Digest,
			Size:   info.Size,
		})
		if err != nil {
			return err
		}
		rc = iohelper.ReadCloser(readerAt)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rc, nil
}

func withOCICaller(ctx context.Context, g session.Group, opt FetchOpt, f func(context.Context, session.Caller) error) error {
	if opt.SessionID != "" {
		timeoutCtx, cancel := context.WithCancelCause(ctx)
		timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded)) //nolint:govet
		defer func() { cancel(errors.WithStack(context.Canceled)) }()

		caller, err := opt.SessionManager.Get(timeoutCtx, opt.SessionID, false)
		if err != nil {
			return err
		}
		return f(ctx, caller)
	}

	return opt.SessionManager.Any(ctx, g, func(ctx context.Context, _ string, caller session.Caller) error {
		return f(ctx, caller)
	})
}
