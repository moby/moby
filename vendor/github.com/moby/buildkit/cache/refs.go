package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/winlayers"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Ref is a reference to cacheable objects.
type Ref interface {
	Mountable
	ID() string
	Release(context.Context) error
	Size(ctx context.Context) (int64, error)
	Metadata() *metadata.StorageItem
	IdentityMapping() *idtools.IdentityMapping
}

type ImmutableRef interface {
	Ref
	Parent() ImmutableRef
	Clone() ImmutableRef

	Info() RefInfo
	Extract(ctx context.Context, s session.Group) error // +progress
	GetRemote(ctx context.Context, createIfNeeded bool, compressionType compression.Type, forceCompression bool, s session.Group) (*solver.Remote, error)
}

type RefInfo struct {
	SnapshotID  string
	ChainID     digest.Digest
	BlobChainID digest.Digest
	DiffID      digest.Digest
	Blob        digest.Digest
	MediaType   string
	Extracted   bool
}

type MutableRef interface {
	Ref
	Commit(context.Context) (ImmutableRef, error)
}

type Mountable interface {
	Mount(ctx context.Context, readonly bool, s session.Group) (snapshot.Mountable, error)
}

type ref interface {
	updateLastUsed() bool
}

type cacheRecord struct {
	cm *cacheManager
	mu *sync.Mutex // the mutex is shared by records sharing data

	mutable bool
	refs    map[ref]struct{}
	parent  *immutableRef
	md      *metadata.StorageItem

	// dead means record is marked as deleted
	dead bool

	view      string
	viewMount snapshot.Mountable

	sizeG flightcontrol.Group

	// these are filled if multiple refs point to same data
	equalMutable   *mutableRef
	equalImmutable *immutableRef

	parentChainCache []digest.Digest
}

// hold ref lock before calling
func (cr *cacheRecord) ref(triggerLastUsed bool, descHandlers DescHandlers) *immutableRef {
	ref := &immutableRef{
		cacheRecord:     cr,
		triggerLastUsed: triggerLastUsed,
		descHandlers:    descHandlers,
	}
	cr.refs[ref] = struct{}{}
	return ref
}

// hold ref lock before calling
func (cr *cacheRecord) mref(triggerLastUsed bool, descHandlers DescHandlers) *mutableRef {
	ref := &mutableRef{
		cacheRecord:     cr,
		triggerLastUsed: triggerLastUsed,
		descHandlers:    descHandlers,
	}
	cr.refs[ref] = struct{}{}
	return ref
}

func (cr *cacheRecord) parentChain() []digest.Digest {
	if cr.parentChainCache != nil {
		return cr.parentChainCache
	}
	blob := getBlob(cr.md)
	if blob == "" {
		return nil
	}

	var parent []digest.Digest
	if cr.parent != nil {
		parent = cr.parent.parentChain()
	}
	pcc := make([]digest.Digest, len(parent)+1)
	copy(pcc, parent)
	pcc[len(parent)] = digest.Digest(blob)
	cr.parentChainCache = pcc
	return pcc
}

// hold ref lock before calling
func (cr *cacheRecord) isDead() bool {
	return cr.dead || (cr.equalImmutable != nil && cr.equalImmutable.dead) || (cr.equalMutable != nil && cr.equalMutable.dead)
}

func (cr *cacheRecord) isLazy(ctx context.Context) (bool, error) {
	if !getBlobOnly(cr.md) {
		return false, nil
	}
	dgst := getBlob(cr.md)
	// special case for moby where there is no compressed blob (empty digest)
	if dgst == "" {
		return false, nil
	}
	_, err := cr.cm.ContentStore.Info(ctx, digest.Digest(dgst))
	if errors.Is(err, errdefs.ErrNotFound) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	// If the snapshot is a remote snapshot, this layer is lazy.
	if info, err := cr.cm.Snapshotter.Stat(ctx, getSnapshotID(cr.md)); err == nil {
		if _, ok := info.Labels["containerd.io/snapshot/remote"]; ok {
			return true, nil
		}
	}

	return false, nil
}

func (cr *cacheRecord) IdentityMapping() *idtools.IdentityMapping {
	return cr.cm.IdentityMapping()
}

func (cr *cacheRecord) Size(ctx context.Context) (int64, error) {
	// this expects that usage() is implemented lazily
	s, err := cr.sizeG.Do(ctx, cr.ID(), func(ctx context.Context) (interface{}, error) {
		cr.mu.Lock()
		s := getSize(cr.md)
		if s != sizeUnknown {
			cr.mu.Unlock()
			return s, nil
		}
		driverID := getSnapshotID(cr.md)
		if cr.equalMutable != nil {
			driverID = getSnapshotID(cr.equalMutable.md)
		}
		cr.mu.Unlock()
		var usage snapshots.Usage
		if !getBlobOnly(cr.md) {
			var err error
			usage, err = cr.cm.ManagerOpt.Snapshotter.Usage(ctx, driverID)
			if err != nil {
				cr.mu.Lock()
				isDead := cr.isDead()
				cr.mu.Unlock()
				if isDead {
					return int64(0), nil
				}
				if !errors.Is(err, errdefs.ErrNotFound) {
					return s, errors.Wrapf(err, "failed to get usage for %s", cr.ID())
				}
			}
		}
		if dgst := getBlob(cr.md); dgst != "" {
			info, err := cr.cm.ContentStore.Info(ctx, digest.Digest(dgst))
			if err == nil {
				usage.Size += info.Size
			}
			for k, v := range info.Labels {
				// accumulate size of compression variant blobs
				if strings.HasPrefix(k, compressionVariantDigestLabelPrefix) {
					if cdgst, err := digest.Parse(v); err == nil {
						if digest.Digest(dgst) == cdgst {
							// do not double count if the label points to this content itself.
							continue
						}
						if info, err := cr.cm.ContentStore.Info(ctx, cdgst); err == nil {
							usage.Size += info.Size
						}
					}
				}
			}
		}
		cr.mu.Lock()
		setSize(cr.md, usage.Size)
		if err := cr.md.Commit(); err != nil {
			cr.mu.Unlock()
			return s, err
		}
		cr.mu.Unlock()
		return usage.Size, nil
	})
	if err != nil {
		return 0, err
	}
	return s.(int64), nil
}

func (cr *cacheRecord) parentRef(hidden bool, descHandlers DescHandlers) *immutableRef {
	p := cr.parent
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ref(hidden, descHandlers)
}

// must be called holding cacheRecord mu
func (cr *cacheRecord) mount(ctx context.Context, readonly bool) (snapshot.Mountable, error) {
	if cr.mutable {
		m, err := cr.cm.Snapshotter.Mounts(ctx, getSnapshotID(cr.md))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to mount %s", cr.ID())
		}
		if readonly {
			m = setReadonly(m)
		}
		return m, nil
	}

	if cr.equalMutable != nil && readonly {
		m, err := cr.cm.Snapshotter.Mounts(ctx, getSnapshotID(cr.equalMutable.md))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to mount %s", cr.equalMutable.ID())
		}
		return setReadonly(m), nil
	}

	if err := cr.finalize(ctx); err != nil {
		return nil, err
	}
	if cr.viewMount == nil { // TODO: handle this better
		view := identity.NewID()
		l, err := cr.cm.LeaseManager.Create(ctx, func(l *leases.Lease) error {
			l.ID = view
			l.Labels = map[string]string{
				"containerd.io/gc.flat": time.Now().UTC().Format(time.RFC3339Nano),
			}
			return nil
		}, leaseutil.MakeTemporary)
		if err != nil {
			return nil, err
		}
		ctx = leases.WithLease(ctx, l.ID)
		m, err := cr.cm.Snapshotter.View(ctx, view, getSnapshotID(cr.md))
		if err != nil {
			cr.cm.LeaseManager.Delete(context.TODO(), leases.Lease{ID: l.ID})
			return nil, errors.Wrapf(err, "failed to mount %s", cr.ID())
		}
		cr.view = view
		cr.viewMount = m
	}
	return cr.viewMount, nil
}

// call when holding the manager lock
func (cr *cacheRecord) remove(ctx context.Context, removeSnapshot bool) error {
	delete(cr.cm.records, cr.ID())
	if cr.parent != nil {
		cr.parent.mu.Lock()
		err := cr.parent.release(ctx)
		cr.parent.mu.Unlock()
		if err != nil {
			return err
		}
	}
	if removeSnapshot {
		if err := cr.cm.LeaseManager.Delete(context.TODO(), leases.Lease{ID: cr.ID()}); err != nil {
			return errors.Wrapf(err, "failed to remove %s", cr.ID())
		}
	}
	if err := cr.cm.md.Clear(cr.ID()); err != nil {
		return err
	}
	return nil
}

func (cr *cacheRecord) ID() string {
	return cr.md.ID()
}

type immutableRef struct {
	*cacheRecord
	triggerLastUsed bool
	descHandlers    DescHandlers
}

type mutableRef struct {
	*cacheRecord
	triggerLastUsed bool
	descHandlers    DescHandlers
}

func (sr *immutableRef) Clone() ImmutableRef {
	sr.mu.Lock()
	ref := sr.ref(false, sr.descHandlers)
	sr.mu.Unlock()
	return ref
}

func (sr *immutableRef) Parent() ImmutableRef {
	if p := sr.parentRef(true, sr.descHandlers); p != nil { // avoid returning typed nil pointer
		return p
	}
	return nil
}

func (sr *immutableRef) Info() RefInfo {
	return RefInfo{
		ChainID:     digest.Digest(getChainID(sr.md)),
		DiffID:      digest.Digest(getDiffID(sr.md)),
		Blob:        digest.Digest(getBlob(sr.md)),
		MediaType:   getMediaType(sr.md),
		BlobChainID: digest.Digest(getBlobChainID(sr.md)),
		SnapshotID:  getSnapshotID(sr.md),
		Extracted:   !getBlobOnly(sr.md),
	}
}

func (sr *immutableRef) ociDesc() (ocispecs.Descriptor, error) {
	desc := ocispecs.Descriptor{
		Digest:      digest.Digest(getBlob(sr.md)),
		Size:        getBlobSize(sr.md),
		MediaType:   getMediaType(sr.md),
		Annotations: make(map[string]string),
	}

	diffID := getDiffID(sr.md)
	if diffID != "" {
		desc.Annotations["containerd.io/uncompressed"] = diffID
	}

	createdAt := GetCreatedAt(sr.md)
	if !createdAt.IsZero() {
		createdAt, err := createdAt.MarshalText()
		if err != nil {
			return ocispecs.Descriptor{}, err
		}
		desc.Annotations["buildkit/createdat"] = string(createdAt)
	}

	return desc, nil
}

const compressionVariantDigestLabelPrefix = "buildkit.io/compression/digest."

func compressionVariantDigestLabel(compressionType compression.Type) string {
	return compressionVariantDigestLabelPrefix + compressionType.String()
}

func (sr *immutableRef) getCompressionBlob(ctx context.Context, compressionType compression.Type) (content.Info, error) {
	cs := sr.cm.ContentStore
	info, err := cs.Info(ctx, digest.Digest(getBlob(sr.md)))
	if err != nil {
		return content.Info{}, err
	}
	dgstS, ok := info.Labels[compressionVariantDigestLabel(compressionType)]
	if ok {
		dgst, err := digest.Parse(dgstS)
		if err != nil {
			return content.Info{}, err
		}
		info, err := cs.Info(ctx, dgst)
		if err != nil {
			return content.Info{}, err
		}
		return info, nil
	}
	return content.Info{}, errdefs.ErrNotFound
}

func (sr *immutableRef) addCompressionBlob(ctx context.Context, dgst digest.Digest, compressionType compression.Type) error {
	cs := sr.cm.ContentStore
	if err := sr.cm.ManagerOpt.LeaseManager.AddResource(ctx, leases.Lease{ID: sr.ID()}, leases.Resource{
		ID:   dgst.String(),
		Type: "content",
	}); err != nil {
		return err
	}
	info, err := cs.Info(ctx, digest.Digest(getBlob(sr.md)))
	if err != nil {
		return err
	}
	if info.Labels == nil {
		info.Labels = make(map[string]string)
	}
	cachedVariantLabel := compressionVariantDigestLabel(compressionType)
	info.Labels[cachedVariantLabel] = dgst.String()
	if _, err := cs.Update(ctx, info, "labels."+cachedVariantLabel); err != nil {
		return err
	}
	return nil
}

// order is from parent->child, sr will be at end of slice
func (sr *immutableRef) parentRefChain() []*immutableRef {
	var count int
	for ref := sr; ref != nil; ref = ref.parent {
		count++
	}
	refs := make([]*immutableRef, count)
	for i, ref := count-1, sr; ref != nil; i, ref = i-1, ref.parent {
		refs[i] = ref
	}
	return refs
}

func (sr *immutableRef) Mount(ctx context.Context, readonly bool, s session.Group) (snapshot.Mountable, error) {
	if err := sr.Extract(ctx, s); err != nil {
		return nil, err
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.cm.Snapshotter.Name() == "stargz" {
		var (
			m    snapshot.Mountable
			rerr error
		)
		if err := sr.withRemoteSnapshotLabelsStargzMode(ctx, s, func() {
			m, rerr = sr.mount(ctx, readonly)
		}); err != nil {
			return nil, err
		}
		return m, rerr
	}

	return sr.mount(ctx, readonly)
}

func (sr *immutableRef) Extract(ctx context.Context, s session.Group) (rerr error) {
	if !getBlobOnly(sr.md) {
		return
	}

	ctx, done, err := leaseutil.WithLease(ctx, sr.cm.LeaseManager, leaseutil.MakeTemporary)
	if err != nil {
		return err
	}
	defer done(ctx)

	if GetLayerType(sr) == "windows" {
		ctx = winlayers.UseWindowsLayerMode(ctx)
	}

	if sr.cm.Snapshotter.Name() == "stargz" {
		if err := sr.withRemoteSnapshotLabelsStargzMode(ctx, s, func() {
			if rerr = sr.prepareRemoteSnapshotsStargzMode(ctx, s); rerr != nil {
				return
			}
			rerr = sr.extract(ctx, sr.descHandlers, s)
		}); err != nil {
			return err
		}
		return rerr
	}

	return sr.extract(ctx, sr.descHandlers, s)
}

func (sr *immutableRef) withRemoteSnapshotLabelsStargzMode(ctx context.Context, s session.Group, f func()) error {
	dhs := sr.descHandlers
	for _, r := range sr.parentRefChain() {
		r := r
		info, err := r.cm.Snapshotter.Stat(ctx, getSnapshotID(r.md))
		if err != nil && !errdefs.IsNotFound(err) {
			return err
		} else if errdefs.IsNotFound(err) {
			continue // This snpashot doesn't exist; skip
		} else if _, ok := info.Labels["containerd.io/snapshot/remote"]; !ok {
			continue // This isn't a remote snapshot; skip
		}
		desc, err := r.ociDesc()
		if err != nil {
			return err
		}
		dh := dhs[desc.Digest]
		if dh == nil {
			continue // no info passed; skip
		}

		// Append temporary labels (based on dh.SnapshotLabels) as hints for remote snapshots.
		// For avoiding collosion among calls, keys of these tmp labels contain an unique ID.
		flds, labels := makeTmpLabelsStargzMode(snapshots.FilterInheritedLabels(dh.SnapshotLabels), s)
		info.Labels = labels
		if _, err := r.cm.Snapshotter.Update(ctx, info, flds...); err != nil {
			return errors.Wrapf(err, "failed to add tmp remote labels for remote snapshot")
		}
		defer func() {
			for k := range info.Labels {
				info.Labels[k] = "" // Remove labels appended in this call
			}
			if _, err := r.cm.Snapshotter.Update(ctx, info, flds...); err != nil {
				logrus.Warn(errors.Wrapf(err, "failed to remove tmp remote labels"))
			}
		}()

		continue
	}

	f()

	return nil
}

func (sr *immutableRef) prepareRemoteSnapshotsStargzMode(ctx context.Context, s session.Group) error {
	_, err := sr.sizeG.Do(ctx, sr.ID()+"-prepare-remote-snapshot", func(ctx context.Context) (_ interface{}, rerr error) {
		dhs := sr.descHandlers
		for _, r := range sr.parentRefChain() {
			r := r
			snapshotID := getSnapshotID(r.md)
			if _, err := r.cm.Snapshotter.Stat(ctx, snapshotID); err == nil {
				continue
			}

			desc, err := r.ociDesc()
			if err != nil {
				return nil, err
			}
			dh := dhs[desc.Digest]
			if dh == nil {
				// We cannot prepare remote snapshots without descHandler.
				return nil, nil
			}

			// tmpLabels contains dh.SnapshotLabels + session IDs. All keys contain
			// an unique ID for avoiding the collision among snapshotter API calls to
			// this snapshot. tmpLabels will be removed at the end of this function.
			defaultLabels := snapshots.FilterInheritedLabels(dh.SnapshotLabels)
			if defaultLabels == nil {
				defaultLabels = make(map[string]string)
			}
			tmpFields, tmpLabels := makeTmpLabelsStargzMode(defaultLabels, s)
			defaultLabels["containerd.io/snapshot.ref"] = snapshotID

			// Prepare remote snapshots
			var (
				key  = fmt.Sprintf("tmp-%s %s", identity.NewID(), r.Info().ChainID)
				opts = []snapshots.Opt{
					snapshots.WithLabels(defaultLabels),
					snapshots.WithLabels(tmpLabels),
				}
			)
			parentID := ""
			if r.parent != nil {
				parentID = getSnapshotID(r.parent.md)
			}
			if err = r.cm.Snapshotter.Prepare(ctx, key, parentID, opts...); err != nil {
				if errdefs.IsAlreadyExists(err) {
					// Check if the targeting snapshot ID has been prepared as
					// a remote snapshot in the snapshotter.
					info, err := r.cm.Snapshotter.Stat(ctx, snapshotID)
					if err == nil { // usable as remote snapshot without unlazying.
						defer func() {
							// Remove tmp labels appended in this func
							for k := range tmpLabels {
								info.Labels[k] = ""
							}
							if _, err := r.cm.Snapshotter.Update(ctx, info, tmpFields...); err != nil {
								logrus.Warn(errors.Wrapf(err,
									"failed to remove tmp remote labels after prepare"))
							}
						}()

						// Try the next layer as well.
						continue
					}
				}
			}

			// This layer and all upper layers cannot be prepared without unlazying.
			break
		}

		return nil, nil
	})
	return err
}

func makeTmpLabelsStargzMode(labels map[string]string, s session.Group) (fields []string, res map[string]string) {
	res = make(map[string]string)
	// Append unique ID to labels for avoiding collision of labels among calls
	id := identity.NewID()
	for k, v := range labels {
		tmpKey := k + "." + id
		fields = append(fields, "labels."+tmpKey)
		res[tmpKey] = v
	}
	for i, sid := range session.AllSessionIDs(s) {
		sidKey := "containerd.io/snapshot/remote/stargz.session." + fmt.Sprintf("%d", i) + "." + id
		fields = append(fields, "labels."+sidKey)
		res[sidKey] = sid
	}
	return
}

func (sr *immutableRef) extract(ctx context.Context, dhs DescHandlers, s session.Group) error {
	_, err := sr.sizeG.Do(ctx, sr.ID()+"-extract", func(ctx context.Context) (_ interface{}, rerr error) {
		snapshotID := getSnapshotID(sr.md)
		if _, err := sr.cm.Snapshotter.Stat(ctx, snapshotID); err == nil {
			return nil, nil
		}

		if sr.cm.Applier == nil {
			return nil, errors.New("extract requires an applier")
		}

		eg, egctx := errgroup.WithContext(ctx)

		parentID := ""
		if sr.parent != nil {
			eg.Go(func() error {
				if err := sr.parent.extract(egctx, dhs, s); err != nil {
					return err
				}
				parentID = getSnapshotID(sr.parent.md)
				return nil
			})
		}

		desc, err := sr.ociDesc()
		if err != nil {
			return nil, err
		}
		dh := dhs[desc.Digest]

		eg.Go(func() error {
			// unlazies if needed, otherwise a no-op
			return lazyRefProvider{
				ref:     sr,
				desc:    desc,
				dh:      dh,
				session: s,
			}.Unlazy(egctx)
		})

		if err := eg.Wait(); err != nil {
			return nil, err
		}

		if dh != nil && dh.Progress != nil {
			_, stopProgress := dh.Progress.Start(ctx)
			defer stopProgress(rerr)
			statusDone := dh.Progress.Status("extracting "+desc.Digest.String(), "extracting")
			defer statusDone()
		}

		key := fmt.Sprintf("extract-%s %s", identity.NewID(), sr.Info().ChainID)

		err = sr.cm.Snapshotter.Prepare(ctx, key, parentID)
		if err != nil {
			return nil, err
		}

		mountable, err := sr.cm.Snapshotter.Mounts(ctx, key)
		if err != nil {
			return nil, err
		}
		mounts, unmount, err := mountable.Mount()
		if err != nil {
			return nil, err
		}
		_, err = sr.cm.Applier.Apply(ctx, desc, mounts)
		if err != nil {
			unmount()
			return nil, err
		}

		if err := unmount(); err != nil {
			return nil, err
		}
		if err := sr.cm.Snapshotter.Commit(ctx, getSnapshotID(sr.md), key); err != nil {
			if !errors.Is(err, errdefs.ErrAlreadyExists) {
				return nil, err
			}
		}
		queueBlobOnly(sr.md, false)
		setSize(sr.md, sizeUnknown)
		if err := sr.md.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

func (sr *immutableRef) Release(ctx context.Context) error {
	sr.cm.mu.Lock()
	defer sr.cm.mu.Unlock()

	sr.mu.Lock()
	defer sr.mu.Unlock()

	return sr.release(ctx)
}

func (sr *immutableRef) updateLastUsed() bool {
	return sr.triggerLastUsed
}

func (sr *immutableRef) updateLastUsedNow() bool {
	if !sr.triggerLastUsed {
		return false
	}
	for r := range sr.refs {
		if r.updateLastUsed() {
			return false
		}
	}
	return true
}

func (sr *immutableRef) release(ctx context.Context) error {
	delete(sr.refs, sr)

	if sr.updateLastUsedNow() {
		updateLastUsed(sr.md)
		if sr.equalMutable != nil {
			sr.equalMutable.triggerLastUsed = true
		}
	}

	if len(sr.refs) == 0 {
		if sr.viewMount != nil { // TODO: release viewMount earlier if possible
			if err := sr.cm.LeaseManager.Delete(ctx, leases.Lease{ID: sr.view}); err != nil {
				return errors.Wrapf(err, "failed to remove view lease %s", sr.view)
			}
			sr.view = ""
			sr.viewMount = nil
		}

		if sr.equalMutable != nil {
			sr.equalMutable.release(ctx)
		}
	}

	return nil
}

func (sr *immutableRef) finalizeLocked(ctx context.Context) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.finalize(ctx)
}

func (cr *cacheRecord) Metadata() *metadata.StorageItem {
	return cr.md
}

// caller must hold cacheRecord.mu
func (cr *cacheRecord) finalize(ctx context.Context) error {
	mutable := cr.equalMutable
	if mutable == nil {
		return nil
	}

	_, err := cr.cm.ManagerOpt.LeaseManager.Create(ctx, func(l *leases.Lease) error {
		l.ID = cr.ID()
		l.Labels = map[string]string{
			"containerd.io/gc.flat": time.Now().UTC().Format(time.RFC3339Nano),
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, errdefs.ErrAlreadyExists) { // migrator adds leases for everything
			return errors.Wrap(err, "failed to create lease")
		}
	}

	if err := cr.cm.ManagerOpt.LeaseManager.AddResource(ctx, leases.Lease{ID: cr.ID()}, leases.Resource{
		ID:   cr.ID(),
		Type: "snapshots/" + cr.cm.ManagerOpt.Snapshotter.Name(),
	}); err != nil {
		cr.cm.LeaseManager.Delete(context.TODO(), leases.Lease{ID: cr.ID()})
		return errors.Wrapf(err, "failed to add snapshot %s to lease", cr.ID())
	}

	err = cr.cm.Snapshotter.Commit(ctx, cr.ID(), mutable.ID())
	if err != nil {
		cr.cm.LeaseManager.Delete(context.TODO(), leases.Lease{ID: cr.ID()})
		return errors.Wrapf(err, "failed to commit %s", mutable.ID())
	}
	mutable.dead = true
	go func() {
		cr.cm.mu.Lock()
		defer cr.cm.mu.Unlock()
		if err := mutable.remove(context.TODO(), true); err != nil {
			logrus.Error(err)
		}
	}()

	cr.equalMutable = nil
	clearEqualMutable(cr.md)
	return cr.md.Commit()
}

func (sr *mutableRef) updateLastUsed() bool {
	return sr.triggerLastUsed
}

func (sr *mutableRef) commit(ctx context.Context) (*immutableRef, error) {
	if !sr.mutable || len(sr.refs) == 0 {
		return nil, errors.Wrapf(errInvalid, "invalid mutable ref %p", sr)
	}

	id := identity.NewID()
	md, _ := sr.cm.md.Get(id)
	rec := &cacheRecord{
		mu:           sr.mu,
		cm:           sr.cm,
		parent:       sr.parentRef(false, sr.descHandlers),
		equalMutable: sr,
		refs:         make(map[ref]struct{}),
		md:           md,
	}

	if descr := GetDescription(sr.md); descr != "" {
		if err := queueDescription(md, descr); err != nil {
			return nil, err
		}
	}

	parentID := ""
	if rec.parent != nil {
		parentID = rec.parent.ID()
	}
	if err := initializeMetadata(rec, parentID); err != nil {
		return nil, err
	}

	sr.cm.records[id] = rec

	if err := sr.md.Commit(); err != nil {
		return nil, err
	}

	queueCommitted(md)
	setSize(md, sizeUnknown)
	setEqualMutable(md, sr.ID())
	if err := md.Commit(); err != nil {
		return nil, err
	}

	ref := rec.ref(true, sr.descHandlers)
	sr.equalImmutable = ref
	return ref, nil
}

func (sr *mutableRef) Mount(ctx context.Context, readonly bool, s session.Group) (snapshot.Mountable, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.cm.Snapshotter.Name() == "stargz" && sr.parent != nil {
		var (
			m    snapshot.Mountable
			rerr error
		)
		if err := sr.parent.withRemoteSnapshotLabelsStargzMode(ctx, s, func() {
			m, rerr = sr.mount(ctx, readonly)
		}); err != nil {
			return nil, err
		}
		return m, rerr
	}

	return sr.mount(ctx, readonly)
}

func (sr *mutableRef) Commit(ctx context.Context) (ImmutableRef, error) {
	sr.cm.mu.Lock()
	defer sr.cm.mu.Unlock()

	sr.mu.Lock()
	defer sr.mu.Unlock()

	return sr.commit(ctx)
}

func (sr *mutableRef) Release(ctx context.Context) error {
	sr.cm.mu.Lock()
	defer sr.cm.mu.Unlock()

	sr.mu.Lock()
	defer sr.mu.Unlock()

	return sr.release(ctx)
}

func (sr *mutableRef) release(ctx context.Context) error {
	delete(sr.refs, sr)
	if getCachePolicy(sr.md) != cachePolicyRetain {
		if sr.equalImmutable != nil {
			if getCachePolicy(sr.equalImmutable.md) == cachePolicyRetain {
				if sr.updateLastUsed() {
					updateLastUsed(sr.md)
					sr.triggerLastUsed = false
				}
				return nil
			}
			if err := sr.equalImmutable.remove(ctx, false); err != nil {
				return err
			}
		}
		return sr.remove(ctx, true)
	}
	if sr.updateLastUsed() {
		updateLastUsed(sr.md)
		sr.triggerLastUsed = false
	}
	return nil
}

func setReadonly(mounts snapshot.Mountable) snapshot.Mountable {
	return &readOnlyMounter{mounts}
}

type readOnlyMounter struct {
	snapshot.Mountable
}

func (m *readOnlyMounter) Mount() ([]mount.Mount, func() error, error) {
	mounts, release, err := m.Mountable.Mount()
	if err != nil {
		return nil, nil, err
	}
	for i, m := range mounts {
		if m.Type == "overlay" {
			mounts[i].Options = readonlyOverlay(m.Options)
			continue
		}
		opts := make([]string, 0, len(m.Options))
		for _, opt := range m.Options {
			if opt != "rw" {
				opts = append(opts, opt)
			}
		}
		opts = append(opts, "ro")
		mounts[i].Options = opts
	}
	return mounts, release, nil
}

func readonlyOverlay(opt []string) []string {
	out := make([]string, 0, len(opt))
	upper := ""
	for _, o := range opt {
		if strings.HasPrefix(o, "upperdir=") {
			upper = strings.TrimPrefix(o, "upperdir=")
		} else if !strings.HasPrefix(o, "workdir=") {
			out = append(out, o)
		}
	}
	if upper != "" {
		for i, o := range out {
			if strings.HasPrefix(o, "lowerdir=") {
				out[i] = "lowerdir=" + upper + ":" + strings.TrimPrefix(o, "lowerdir=")
			}
		}
	}
	return out
}
