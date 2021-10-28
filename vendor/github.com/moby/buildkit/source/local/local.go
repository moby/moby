package local

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/bklog"

	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/contenthash"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Opt struct {
	CacheAccessor cache.Accessor
}

func NewSource(opt Opt) (source.Source, error) {
	ls := &localSource{
		cm: opt.CacheAccessor,
	}
	return ls, nil
}

type localSource struct {
	cm cache.Accessor
}

func (ls *localSource) ID() string {
	return srctypes.LocalScheme
}

func (ls *localSource) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, _ solver.Vertex) (source.SourceInstance, error) {
	localIdentifier, ok := id.(*source.LocalIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid local identifier %v", id)
	}

	return &localSourceHandler{
		src:         *localIdentifier,
		sm:          sm,
		localSource: ls,
	}, nil
}

type localSourceHandler struct {
	src source.LocalIdentifier
	sm  *session.Manager
	*localSource
}

func (ls *localSourceHandler) CacheKey(ctx context.Context, g session.Group, index int) (string, string, solver.CacheOpts, bool, error) {
	sessionID := ls.src.SessionID

	if sessionID == "" {
		id := g.SessionIterator().NextSession()
		if id == "" {
			return "", "", nil, false, errors.New("could not access local files without session")
		}
		sessionID = id
	}
	dt, err := json.Marshal(struct {
		SessionID       string
		IncludePatterns []string
		ExcludePatterns []string
		FollowPaths     []string
	}{SessionID: sessionID, IncludePatterns: ls.src.IncludePatterns, ExcludePatterns: ls.src.ExcludePatterns, FollowPaths: ls.src.FollowPaths})
	if err != nil {
		return "", "", nil, false, err
	}
	return "session:" + ls.src.Name + ":" + digest.FromBytes(dt).String(), digest.FromBytes(dt).String(), nil, true, nil
}

func (ls *localSourceHandler) Snapshot(ctx context.Context, g session.Group) (cache.ImmutableRef, error) {
	var ref cache.ImmutableRef
	err := ls.sm.Any(ctx, g, func(ctx context.Context, _ string, c session.Caller) error {
		r, err := ls.snapshot(ctx, g, c)
		if err != nil {
			return err
		}
		ref = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func (ls *localSourceHandler) snapshot(ctx context.Context, s session.Group, caller session.Caller) (out cache.ImmutableRef, retErr error) {
	sharedKey := ls.src.Name + ":" + ls.src.SharedKeyHint + ":" + caller.SharedKey() // TODO: replace caller.SharedKey() with source based hint from client(absolute-path+nodeid)

	var mutable cache.MutableRef
	sis, err := searchSharedKey(ctx, ls.cm, sharedKey)
	if err != nil {
		return nil, err
	}
	for _, si := range sis {
		if m, err := ls.cm.GetMutable(ctx, si.ID()); err == nil {
			bklog.G(ctx).Debugf("reusing ref for local: %s", m.ID())
			mutable = m
			break
		}
	}

	if mutable == nil {
		m, err := ls.cm.New(ctx, nil, s, cache.CachePolicyRetain, cache.WithRecordType(client.UsageRecordTypeLocalSource), cache.WithDescription(fmt.Sprintf("local source for %s", ls.src.Name)))
		if err != nil {
			return nil, err
		}
		mutable = m
		bklog.G(ctx).Debugf("new ref for local: %s", mutable.ID())
	}

	defer func() {
		if retErr != nil && mutable != nil {
			// on error remove the record as checksum update is in undefined state
			if err := mutable.SetCachePolicyDefault(); err != nil {
				bklog.G(ctx).Errorf("failed to reset mutable cachepolicy: %v", err)
			}
			contenthash.ClearCacheContext(mutable)
			go mutable.Release(context.TODO())
		}
	}()

	mount, err := mutable.Mount(ctx, false, s)
	if err != nil {
		return nil, err
	}

	lm := snapshot.LocalMounter(mount)

	dest, err := lm.Mount()
	if err != nil {
		return nil, err
	}

	defer func() {
		if retErr != nil && lm != nil {
			lm.Unmount()
		}
	}()

	cc, err := contenthash.GetCacheContext(ctx, mutable)
	if err != nil {
		return nil, err
	}

	opt := filesync.FSSendRequestOpt{
		Name:             ls.src.Name,
		IncludePatterns:  ls.src.IncludePatterns,
		ExcludePatterns:  ls.src.ExcludePatterns,
		FollowPaths:      ls.src.FollowPaths,
		OverrideExcludes: false,
		DestDir:          dest,
		CacheUpdater:     &cacheUpdater{cc, mount.IdentityMapping()},
		ProgressCb:       newProgressHandler(ctx, "transferring "+ls.src.Name+":"),
		Differ:           ls.src.Differ,
	}

	if idmap := mount.IdentityMapping(); idmap != nil {
		opt.Filter = func(p string, stat *fstypes.Stat) bool {
			identity, err := idmap.ToHost(idtools.Identity{
				UID: int(stat.Uid),
				GID: int(stat.Gid),
			})
			if err != nil {
				return false
			}
			stat.Uid = uint32(identity.UID)
			stat.Gid = uint32(identity.GID)
			return true
		}
	}

	if err := filesync.FSSync(ctx, caller, opt); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errors.Errorf("local source %s not enabled from the client", ls.src.Name)
		}
		return nil, err
	}

	if err := lm.Unmount(); err != nil {
		return nil, err
	}
	lm = nil

	if err := contenthash.SetCacheContext(ctx, mutable, cc); err != nil {
		return nil, err
	}

	// skip storing snapshot by the shared key if it already exists
	md := cacheRefMetadata{mutable}
	if md.getSharedKey() != sharedKey {
		if err := md.setSharedKey(sharedKey); err != nil {
			return nil, err
		}
		bklog.G(ctx).Debugf("saved %s as %s", mutable.ID(), sharedKey)
	}

	snap, err := mutable.Commit(ctx)
	if err != nil {
		return nil, err
	}

	mutable = nil // avoid deferred cleanup

	return snap, nil
}

func newProgressHandler(ctx context.Context, id string) func(int, bool) {
	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	pw, _, _ := progress.NewFromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
		Action:  "transferring",
	}
	pw.Write(id, st)
	return func(s int, last bool) {
		if last || limiter.Allow() {
			st.Current = s
			if last {
				now := time.Now()
				st.Completed = &now
			}
			pw.Write(id, st)
			if last {
				pw.Close()
			}
		}
	}
}

type cacheUpdater struct {
	contenthash.CacheContext
	idmap *idtools.IdentityMapping
}

func (cu *cacheUpdater) MarkSupported(bool) {
}

func (cu *cacheUpdater) ContentHasher() fsutil.ContentHasher {
	return contenthash.NewFromStat
}

const keySharedKey = "local.sharedKey"
const sharedKeyIndex = keySharedKey + ":"

func searchSharedKey(ctx context.Context, store cache.MetadataStore, k string) ([]cacheRefMetadata, error) {
	var results []cacheRefMetadata
	mds, err := store.Search(ctx, sharedKeyIndex+k)
	if err != nil {
		return nil, err
	}
	for _, md := range mds {
		results = append(results, cacheRefMetadata{md})
	}
	return results, nil
}

type cacheRefMetadata struct {
	cache.RefMetadata
}

func (md cacheRefMetadata) getSharedKey() string {
	return md.GetString(keySharedKey)
}

func (md cacheRefMetadata) setSharedKey(key string) error {
	return md.SetString(keySharedKey, key, sharedKeyIndex+key)
}
