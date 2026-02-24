package containerblob

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/v2/core/remotes"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	sessioncontent "github.com/moby/buildkit/session/content"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/iohelper"
	"github.com/moby/buildkit/util/resolver"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type puller struct {
	src *Source
	id  *ImageBlobIdentifier

	SessionManager *session.Manager

	rc   io.ReadCloser
	dgst digest.Digest
}

func (p *puller) hash() (digest.Digest, error) {
	dgst := p.id.Reference.Digest()
	if err := dgst.Validate(); err != nil {
		return "", err
	}

	dt, err := json.Marshal(struct {
		Digest         digest.Digest
		Filename       string
		Perm, UID, GID int
	}{
		Digest:   dgst,
		Filename: p.id.Filename,
		Perm:     p.id.Perm,
		UID:      p.id.UID,
		GID:      p.id.GID,
	})
	if err != nil {
		return "", err
	}
	return digest.FromBytes(dt), nil
}

func (p *puller) ensureResolver(ctx context.Context, g session.Group) error {
	if p.rc != nil {
		return nil
	}
	dgst := p.id.Reference.Digest()
	if err := dgst.Validate(); err != nil {
		return errors.Wrap(err, "invalid reference digest")
	}

	var (
		rc  io.ReadCloser
		err error
	)
	if p.id.Scheme() == srctypes.OCIBlobScheme {
		rc, err = p.fetchFromOCILayoutStore(ctx, g, dgst)
		if err != nil {
			return err
		}
	} else {
		r := resolver.DefaultPool.GetResolver(p.src.RegistryHosts, p.id.Reference.String(), resolver.ScopeType{}, p.SessionManager, g)
		f, err := r.Fetcher(ctx, p.id.Reference.String())
		if err != nil {
			return err
		}

		fd, ok := f.(remotes.FetcherByDigest)
		if !ok {
			return errors.Errorf("invalid blob fetcher: %T", f)
		}

		rc, _, err = fd.FetchByDigest(ctx, dgst)
		if err != nil {
			return err
		}
	}

	p.rc = rc
	p.dgst = dgst
	return nil
}

func (p *puller) fetchFromOCILayoutStore(ctx context.Context, g session.Group, dgst digest.Digest) (io.ReadCloser, error) {
	if p.id.StoreID == "" {
		return nil, errors.Errorf("oci-layout blob source requires store id")
	}

	var rc io.ReadCloser
	err := p.withOCICaller(ctx, g, func(ctx context.Context, caller session.Caller) error {
		store := sessioncontent.NewCallerStore(caller, "oci:"+p.id.StoreID)
		info, err := store.Info(ctx, dgst)
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

func (p *puller) withOCICaller(ctx context.Context, g session.Group, f func(context.Context, session.Caller) error) error {
	if p.id.SessionID != "" {
		timeoutCtx, cancel := context.WithCancelCause(ctx)
		timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded)) //nolint:govet
		defer func() { cancel(errors.WithStack(context.Canceled)) }()

		caller, err := p.SessionManager.Get(timeoutCtx, p.id.SessionID, false)
		if err != nil {
			return err
		}
		return f(ctx, caller)
	}

	return p.SessionManager.Any(ctx, g, func(ctx context.Context, _ string, caller session.Caller) error {
		return f(ctx, caller)
	})
}

func (p *puller) CacheKey(ctx context.Context, jobCtx solver.JobContext, index int) (cacheKey string, imgDigest string, cacheOpts solver.CacheOpts, cacheDone bool, err error) {
	dgst := p.id.Reference.Digest()
	if err := dgst.Validate(); err != nil {
		return "", "", nil, false, errors.Wrap(err, "invalid reference digest")
	}

	info, err := p.src.ContentStore.Info(ctx, dgst)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			return "", "", nil, false, err
		}
	}

	if ok, err := contentutil.HasSource(info, p.id.Reference); err == nil && ok {
		h, err := p.hash()
		if err != nil {
			return "", "", nil, false, err
		}
		return h.String(), dgst.String(), nil, true, nil
	}

	h, err := p.hash()
	if err != nil {
		return "", "", nil, false, err
	}
	return h.String(), dgst.String(), nil, true, nil
}

func (p *puller) Snapshot(ctx context.Context, jobCtx solver.JobContext) (ir cache.ImmutableRef, err error) {
	var g session.Group
	if jobCtx != nil {
		g = jobCtx.Session()
	}

	if err := p.ensureResolver(ctx, g); err != nil {
		return nil, err
	}
	defer func() {
		if p.rc != nil {
			p.rc.Close()
			p.rc = nil
		}
	}()

	newRef, err := p.src.CacheAccessor.New(ctx, nil, g, cache.CachePolicyRetain, cache.WithDescription(fmt.Sprintf("blob %s", p.id.Reference.String())))
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil && newRef != nil {
			newRef.Release(context.WithoutCancel(ctx))
		}
	}()

	mount, err := newRef.Mount(ctx, false, g)
	if err != nil {
		return nil, err
	}

	lm := snapshot.LocalMounter(mount)
	dir, err := lm.Mount()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && lm != nil {
			lm.Unmount()
		}
	}()

	perm := 0600
	if p.id.Perm != 0 {
		perm = p.id.Perm
	}
	fn := p.id.Filename
	if fn == "" {
		fn = p.dgst.Hex()
	}

	fp := filepath.Join(dir, fn)
	f, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(perm))
	if err != nil {
		return nil, err
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), p.rc); err != nil {
		return nil, err
	}

	if err := f.Close(); err != nil {
		return nil, err
	}
	f = nil

	uid := p.id.UID
	gid := p.id.GID
	if idmap := mount.IdentityMapping(); idmap != nil {
		uid, gid, err = idmap.ToHost(uid, gid)
		if err != nil {
			return nil, err
		}
	}
	if gid != 0 || uid != 0 {
		if err := os.Chown(fp, uid, gid); err != nil {
			return nil, err
		}
	}

	mTime := time.Unix(0, 0)
	if err := os.Chtimes(fp, mTime, mTime); err != nil {
		return nil, err
	}

	lm.Unmount()
	lm = nil

	ref, err := newRef.Commit(ctx)
	if err != nil {
		return nil, err
	}
	newRef = nil
	return ref, nil
}
