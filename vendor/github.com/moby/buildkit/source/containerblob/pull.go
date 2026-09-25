package containerblob

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/source/containerblob/blobfetch"
	"github.com/moby/buildkit/source/util/pathutil"
	"github.com/moby/buildkit/util/contentutil"
	digest "github.com/opencontainers/go-digest"
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

	rc, _, err := blobfetch.FetchBlob(ctx, g, blobfetch.FetchOpt{
		Scheme:         p.id.Scheme(),
		Ref:            p.id.Reference.String(),
		Digest:         dgst,
		RegistryHosts:  p.src.RegistryHosts,
		SessionManager: p.SessionManager,
		SessionID:      p.id.SessionID,
		StoreID:        p.id.StoreID,
	})
	if err != nil {
		return err
	}

	p.rc = rc
	p.dgst = dgst
	return nil
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
	} else {
		fn = pathutil.SafeFileName(fn)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	f, err := root.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(perm))
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
		if err := root.Chown(fn, uid, gid); err != nil {
			return nil, err
		}
	}

	mTime := time.Unix(0, 0)
	if err := root.Chtimes(fn, mTime, mTime); err != nil {
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
