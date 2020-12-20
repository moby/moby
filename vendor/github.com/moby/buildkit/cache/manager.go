package cache

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/gc"
	"github.com/containerd/containerd/leases"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/flightcontrol"
	digest "github.com/opencontainers/go-digest"
	imagespecidentity "github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var (
	ErrLocked   = errors.New("locked")
	errNotFound = errors.New("not found")
	errInvalid  = errors.New("invalid")
)

type ManagerOpt struct {
	Snapshotter     snapshot.Snapshotter
	MetadataStore   *metadata.Store
	ContentStore    content.Store
	LeaseManager    leases.Manager
	PruneRefChecker ExternalRefCheckerFunc
	GarbageCollect  func(ctx context.Context) (gc.Stats, error)
	Applier         diff.Applier
	Differ          diff.Comparer
}

type Accessor interface {
	GetByBlob(ctx context.Context, desc ocispec.Descriptor, parent ImmutableRef, opts ...RefOption) (ImmutableRef, error)
	Get(ctx context.Context, id string, opts ...RefOption) (ImmutableRef, error)

	New(ctx context.Context, parent ImmutableRef, s session.Group, opts ...RefOption) (MutableRef, error)
	GetMutable(ctx context.Context, id string, opts ...RefOption) (MutableRef, error) // Rebase?
	IdentityMapping() *idtools.IdentityMapping
	Metadata(string) *metadata.StorageItem
}

type Controller interface {
	DiskUsage(ctx context.Context, info client.DiskUsageInfo) ([]*client.UsageInfo, error)
	Prune(ctx context.Context, ch chan client.UsageInfo, info ...client.PruneInfo) error
}

type Manager interface {
	Accessor
	Controller
	Close() error
}

type ExternalRefCheckerFunc func() (ExternalRefChecker, error)

type ExternalRefChecker interface {
	Exists(string, []digest.Digest) bool
}

type cacheManager struct {
	records map[string]*cacheRecord
	mu      sync.Mutex
	ManagerOpt
	md *metadata.Store

	muPrune sync.Mutex // make sure parallel prune is not allowed so there will not be inconsistent results
	unlazyG flightcontrol.Group
}

func NewManager(opt ManagerOpt) (Manager, error) {
	cm := &cacheManager{
		ManagerOpt: opt,
		md:         opt.MetadataStore,
		records:    make(map[string]*cacheRecord),
	}

	if err := cm.init(context.TODO()); err != nil {
		return nil, err
	}

	// cm.scheduleGC(5 * time.Minute)

	return cm, nil
}

func (cm *cacheManager) GetByBlob(ctx context.Context, desc ocispec.Descriptor, parent ImmutableRef, opts ...RefOption) (ir ImmutableRef, rerr error) {
	diffID, err := diffIDFromDescriptor(desc)
	if err != nil {
		return nil, err
	}
	chainID := diffID
	blobChainID := imagespecidentity.ChainID([]digest.Digest{desc.Digest, diffID})

	descHandlers := descHandlersOf(opts...)
	if desc.Digest != "" && (descHandlers == nil || descHandlers[desc.Digest] == nil) {
		if _, err := cm.ContentStore.Info(ctx, desc.Digest); errors.Is(err, errdefs.ErrNotFound) {
			return nil, NeedsRemoteProvidersError([]digest.Digest{desc.Digest})
		} else if err != nil {
			return nil, err
		}
	}

	var p *immutableRef
	var parentID string
	if parent != nil {
		pInfo := parent.Info()
		if pInfo.ChainID == "" || pInfo.BlobChainID == "" {
			return nil, errors.Errorf("failed to get ref by blob on non-addressable parent")
		}
		chainID = imagespecidentity.ChainID([]digest.Digest{pInfo.ChainID, chainID})
		blobChainID = imagespecidentity.ChainID([]digest.Digest{pInfo.BlobChainID, blobChainID})

		p2, err := cm.Get(ctx, parent.ID(), NoUpdateLastUsed, descHandlers)
		if err != nil {
			return nil, err
		}
		if err := p2.Finalize(ctx, true); err != nil {
			return nil, err
		}
		parentID = p2.ID()
		p = p2.(*immutableRef)
	}

	releaseParent := false
	defer func() {
		if releaseParent || rerr != nil && p != nil {
			p.Release(context.TODO())
		}
	}()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	sis, err := cm.MetadataStore.Search("blobchainid:" + blobChainID.String())
	if err != nil {
		return nil, err
	}

	if len(sis) > 0 {
		ref, err := cm.get(ctx, sis[0].ID(), opts...)
		if err != nil && !IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to get record %s by blobchainid", sis[0].ID())
		}
		if p != nil {
			releaseParent = true
		}
		if err := setImageRefMetadata(ref, opts...); err != nil {
			return nil, errors.Wrapf(err, "failed to append image ref metadata to ref %s", ref.ID())
		}
		return ref, nil
	}

	sis, err = cm.MetadataStore.Search("chainid:" + chainID.String())
	if err != nil {
		return nil, err
	}

	var link ImmutableRef
	if len(sis) > 0 {
		ref, err := cm.get(ctx, sis[0].ID(), opts...)
		if err != nil && !IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to get record %s by chainid", sis[0].ID())
		}
		link = ref
	}

	id := identity.NewID()
	snapshotID := chainID.String()
	blobOnly := true
	if link != nil {
		snapshotID = getSnapshotID(link.Metadata())
		blobOnly = getBlobOnly(link.Metadata())
		go link.Release(context.TODO())
	}

	l, err := cm.ManagerOpt.LeaseManager.Create(ctx, func(l *leases.Lease) error {
		l.ID = id
		l.Labels = map[string]string{
			"containerd.io/gc.flat": time.Now().UTC().Format(time.RFC3339Nano),
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create lease")
	}

	defer func() {
		if rerr != nil {
			if err := cm.ManagerOpt.LeaseManager.Delete(context.TODO(), leases.Lease{
				ID: l.ID,
			}); err != nil {
				logrus.Errorf("failed to remove lease: %+v", err)
			}
		}
	}()

	if err := cm.ManagerOpt.LeaseManager.AddResource(ctx, l, leases.Resource{
		ID:   snapshotID,
		Type: "snapshots/" + cm.ManagerOpt.Snapshotter.Name(),
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to add snapshot %s to lease", id)
	}

	if desc.Digest != "" {
		if err := cm.ManagerOpt.LeaseManager.AddResource(ctx, leases.Lease{ID: id}, leases.Resource{
			ID:   desc.Digest.String(),
			Type: "content",
		}); err != nil {
			return nil, errors.Wrapf(err, "failed to add blob %s to lease", id)
		}
	}

	md, _ := cm.md.Get(id)

	rec := &cacheRecord{
		mu:     &sync.Mutex{},
		cm:     cm,
		refs:   make(map[ref]struct{}),
		parent: p,
		md:     md,
	}

	if err := initializeMetadata(rec, parentID, opts...); err != nil {
		return nil, err
	}

	if err := setImageRefMetadata(rec, opts...); err != nil {
		return nil, errors.Wrapf(err, "failed to append image ref metadata to ref %s", rec.ID())
	}

	queueDiffID(rec.md, diffID.String())
	queueBlob(rec.md, desc.Digest.String())
	queueChainID(rec.md, chainID.String())
	queueBlobChainID(rec.md, blobChainID.String())
	queueSnapshotID(rec.md, snapshotID)
	queueBlobOnly(rec.md, blobOnly)
	queueMediaType(rec.md, desc.MediaType)
	queueBlobSize(rec.md, desc.Size)
	queueCommitted(rec.md)

	if err := rec.md.Commit(); err != nil {
		return nil, err
	}

	cm.records[id] = rec

	return rec.ref(true, descHandlers), nil
}

// init loads all snapshots from metadata state and tries to load the records
// from the snapshotter. If snaphot can't be found, metadata is deleted as well.
func (cm *cacheManager) init(ctx context.Context) error {
	items, err := cm.md.All()
	if err != nil {
		return err
	}

	for _, si := range items {
		if _, err := cm.getRecord(ctx, si.ID()); err != nil {
			logrus.Debugf("could not load snapshot %s: %+v", si.ID(), err)
			cm.md.Clear(si.ID())
			cm.LeaseManager.Delete(ctx, leases.Lease{ID: si.ID()})
		}
	}
	return nil
}

// IdentityMapping returns the userns remapping used for refs
func (cm *cacheManager) IdentityMapping() *idtools.IdentityMapping {
	return cm.Snapshotter.IdentityMapping()
}

// Close closes the manager and releases the metadata database lock. No other
// method should be called after Close.
func (cm *cacheManager) Close() error {
	// TODO: allocate internal context and cancel it here
	return cm.md.Close()
}

// Get returns an immutable snapshot reference for ID
func (cm *cacheManager) Get(ctx context.Context, id string, opts ...RefOption) (ImmutableRef, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.get(ctx, id, opts...)
}

func (cm *cacheManager) Metadata(id string) *metadata.StorageItem {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	r, ok := cm.records[id]
	if !ok {
		return nil
	}
	return r.Metadata()
}

// get requires manager lock to be taken
func (cm *cacheManager) get(ctx context.Context, id string, opts ...RefOption) (*immutableRef, error) {
	rec, err := cm.getRecord(ctx, id, opts...)
	if err != nil {
		return nil, err
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()

	triggerUpdate := true
	for _, o := range opts {
		if o == NoUpdateLastUsed {
			triggerUpdate = false
		}
	}

	descHandlers := descHandlersOf(opts...)

	if rec.mutable {
		if len(rec.refs) != 0 {
			return nil, errors.Wrapf(ErrLocked, "%s is locked", id)
		}
		if rec.equalImmutable != nil {
			return rec.equalImmutable.ref(triggerUpdate, descHandlers), nil
		}
		return rec.mref(triggerUpdate, descHandlers).commit(ctx)
	}

	return rec.ref(triggerUpdate, descHandlers), nil
}

// getRecord returns record for id. Requires manager lock.
func (cm *cacheManager) getRecord(ctx context.Context, id string, opts ...RefOption) (cr *cacheRecord, retErr error) {
	checkLazyProviders := func(rec *cacheRecord) error {
		missing := NeedsRemoteProvidersError(nil)
		dhs := descHandlersOf(opts...)
		for {
			blob := digest.Digest(getBlob(rec.md))
			if isLazy, err := rec.isLazy(ctx); err != nil {
				return err
			} else if isLazy && dhs[blob] == nil {
				missing = append(missing, blob)
			}

			if rec.parent == nil {
				break
			}
			rec = rec.parent.cacheRecord
		}
		if len(missing) > 0 {
			return missing
		}
		return nil
	}

	if rec, ok := cm.records[id]; ok {
		if rec.isDead() {
			return nil, errors.Wrapf(errNotFound, "failed to get dead record %s", id)
		}
		if err := checkLazyProviders(rec); err != nil {
			return nil, err
		}
		return rec, nil
	}

	md, ok := cm.md.Get(id)
	if !ok {
		return nil, errors.Wrapf(errNotFound, "%s not found", id)
	}
	if mutableID := getEqualMutable(md); mutableID != "" {
		mutable, err := cm.getRecord(ctx, mutableID)
		if err != nil {
			// check loading mutable deleted record from disk
			if IsNotFound(err) {
				cm.md.Clear(id)
			}
			return nil, err
		}

		// parent refs are possibly lazy so keep it hold the description handlers.
		var dhs DescHandlers
		if mutable.parent != nil {
			dhs = mutable.parent.descHandlers
		}
		rec := &cacheRecord{
			mu:           &sync.Mutex{},
			cm:           cm,
			refs:         make(map[ref]struct{}),
			parent:       mutable.parentRef(false, dhs),
			md:           md,
			equalMutable: &mutableRef{cacheRecord: mutable},
		}
		mutable.equalImmutable = &immutableRef{cacheRecord: rec}
		cm.records[id] = rec
		return rec, nil
	}

	var parent *immutableRef
	if parentID := getParent(md); parentID != "" {
		var err error
		parent, err = cm.get(ctx, parentID, append(opts, NoUpdateLastUsed)...)
		if err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				parent.mu.Lock()
				parent.release(context.TODO())
				parent.mu.Unlock()
			}
		}()
	}

	rec := &cacheRecord{
		mu:      &sync.Mutex{},
		mutable: !getCommitted(md),
		cm:      cm,
		refs:    make(map[ref]struct{}),
		parent:  parent,
		md:      md,
	}

	// the record was deleted but we crashed before data on disk was removed
	if getDeleted(md) {
		if err := rec.remove(ctx, true); err != nil {
			return nil, err
		}
		return nil, errors.Wrapf(errNotFound, "failed to get deleted record %s", id)
	}

	if err := initializeMetadata(rec, getParent(md), opts...); err != nil {
		return nil, err
	}

	if err := setImageRefMetadata(rec, opts...); err != nil {
		return nil, errors.Wrapf(err, "failed to append image ref metadata to ref %s", rec.ID())
	}

	cm.records[id] = rec
	if err := checkLazyProviders(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (cm *cacheManager) New(ctx context.Context, s ImmutableRef, sess session.Group, opts ...RefOption) (mr MutableRef, err error) {
	id := identity.NewID()

	var parent *immutableRef
	var parentID string
	var parentSnapshotID string
	if s != nil {
		if _, ok := s.(*immutableRef); ok {
			parent = s.Clone().(*immutableRef)
		} else {
			p, err := cm.Get(ctx, s.ID(), append(opts, NoUpdateLastUsed)...)
			if err != nil {
				return nil, err
			}
			parent = p.(*immutableRef)
		}
		if err := parent.Finalize(ctx, true); err != nil {
			return nil, err
		}
		if err := parent.Extract(ctx, sess); err != nil {
			return nil, err
		}
		parentSnapshotID = getSnapshotID(parent.md)
		parentID = parent.ID()
	}

	defer func() {
		if err != nil && parent != nil {
			parent.Release(context.TODO())
		}
	}()

	l, err := cm.ManagerOpt.LeaseManager.Create(ctx, func(l *leases.Lease) error {
		l.ID = id
		l.Labels = map[string]string{
			"containerd.io/gc.flat": time.Now().UTC().Format(time.RFC3339Nano),
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create lease")
	}

	defer func() {
		if err != nil {
			if err := cm.ManagerOpt.LeaseManager.Delete(context.TODO(), leases.Lease{
				ID: l.ID,
			}); err != nil {
				logrus.Errorf("failed to remove lease: %+v", err)
			}
		}
	}()

	if err := cm.ManagerOpt.LeaseManager.AddResource(ctx, l, leases.Resource{
		ID:   id,
		Type: "snapshots/" + cm.ManagerOpt.Snapshotter.Name(),
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to add snapshot %s to lease", id)
	}

	if err := cm.Snapshotter.Prepare(ctx, id, parentSnapshotID); err != nil {
		return nil, errors.Wrapf(err, "failed to prepare %s", id)
	}

	md, _ := cm.md.Get(id)

	rec := &cacheRecord{
		mu:      &sync.Mutex{},
		mutable: true,
		cm:      cm,
		refs:    make(map[ref]struct{}),
		parent:  parent,
		md:      md,
	}

	if err := initializeMetadata(rec, parentID, opts...); err != nil {
		return nil, err
	}

	if err := setImageRefMetadata(rec, opts...); err != nil {
		return nil, errors.Wrapf(err, "failed to append image ref metadata to ref %s", rec.ID())
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.records[id] = rec // TODO: save to db

	// parent refs are possibly lazy so keep it hold the description handlers.
	var dhs DescHandlers
	if parent != nil {
		dhs = parent.descHandlers
	}
	return rec.mref(true, dhs), nil
}

func (cm *cacheManager) GetMutable(ctx context.Context, id string, opts ...RefOption) (MutableRef, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	rec, err := cm.getRecord(ctx, id, opts...)
	if err != nil {
		return nil, err
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if !rec.mutable {
		return nil, errors.Wrapf(errInvalid, "%s is not mutable", id)
	}

	if len(rec.refs) != 0 {
		return nil, errors.Wrapf(ErrLocked, "%s is locked", id)
	}

	if rec.equalImmutable != nil {
		if len(rec.equalImmutable.refs) != 0 {
			return nil, errors.Wrapf(ErrLocked, "%s is locked", id)
		}
		delete(cm.records, rec.equalImmutable.ID())
		if err := rec.equalImmutable.remove(ctx, false); err != nil {
			return nil, err
		}
		rec.equalImmutable = nil
	}

	return rec.mref(true, descHandlersOf(opts...)), nil
}

func (cm *cacheManager) Prune(ctx context.Context, ch chan client.UsageInfo, opts ...client.PruneInfo) error {
	cm.muPrune.Lock()

	for _, opt := range opts {
		if err := cm.pruneOnce(ctx, ch, opt); err != nil {
			cm.muPrune.Unlock()
			return err
		}
	}

	cm.muPrune.Unlock()

	if cm.GarbageCollect != nil {
		if _, err := cm.GarbageCollect(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (cm *cacheManager) pruneOnce(ctx context.Context, ch chan client.UsageInfo, opt client.PruneInfo) error {
	filter, err := filters.ParseAll(opt.Filter...)
	if err != nil {
		return errors.Wrapf(err, "failed to parse prune filters %v", opt.Filter)
	}

	var check ExternalRefChecker
	if f := cm.PruneRefChecker; f != nil && (!opt.All || len(opt.Filter) > 0) {
		c, err := f()
		if err != nil {
			return errors.WithStack(err)
		}
		check = c
	}

	totalSize := int64(0)
	if opt.KeepBytes != 0 {
		du, err := cm.DiskUsage(ctx, client.DiskUsageInfo{})
		if err != nil {
			return err
		}
		for _, ui := range du {
			if ui.Shared {
				continue
			}
			totalSize += ui.Size
		}
	}

	return cm.prune(ctx, ch, pruneOpt{
		filter:       filter,
		all:          opt.All,
		checkShared:  check,
		keepDuration: opt.KeepDuration,
		keepBytes:    opt.KeepBytes,
		totalSize:    totalSize,
	})
}

func (cm *cacheManager) prune(ctx context.Context, ch chan client.UsageInfo, opt pruneOpt) error {
	var toDelete []*deleteRecord

	if opt.keepBytes != 0 && opt.totalSize < opt.keepBytes {
		return nil
	}

	cm.mu.Lock()

	gcMode := opt.keepBytes != 0
	cutOff := time.Now().Add(-opt.keepDuration)

	locked := map[*sync.Mutex]struct{}{}

	for _, cr := range cm.records {
		if _, ok := locked[cr.mu]; ok {
			continue
		}
		cr.mu.Lock()

		// ignore duplicates that share data
		if cr.equalImmutable != nil && len(cr.equalImmutable.refs) > 0 || cr.equalMutable != nil && len(cr.refs) == 0 {
			cr.mu.Unlock()
			continue
		}

		if cr.isDead() {
			cr.mu.Unlock()
			continue
		}

		if len(cr.refs) == 0 {
			recordType := GetRecordType(cr)
			if recordType == "" {
				recordType = client.UsageRecordTypeRegular
			}

			shared := false
			if opt.checkShared != nil {
				shared = opt.checkShared.Exists(cr.ID(), cr.parentChain())
			}

			if !opt.all {
				if recordType == client.UsageRecordTypeInternal || recordType == client.UsageRecordTypeFrontend || shared {
					cr.mu.Unlock()
					continue
				}
			}

			c := &client.UsageInfo{
				ID:         cr.ID(),
				Mutable:    cr.mutable,
				RecordType: recordType,
				Shared:     shared,
			}

			usageCount, lastUsedAt := getLastUsed(cr.md)
			c.LastUsedAt = lastUsedAt
			c.UsageCount = usageCount

			if opt.keepDuration != 0 {
				if lastUsedAt != nil && lastUsedAt.After(cutOff) {
					cr.mu.Unlock()
					continue
				}
			}

			if opt.filter.Match(adaptUsageInfo(c)) {
				toDelete = append(toDelete, &deleteRecord{
					cacheRecord: cr,
					lastUsedAt:  c.LastUsedAt,
					usageCount:  c.UsageCount,
				})
				if !gcMode {
					cr.dead = true

					// mark metadata as deleted in case we crash before cleanup finished
					if err := setDeleted(cr.md); err != nil {
						cr.mu.Unlock()
						cm.mu.Unlock()
						return err
					}
				} else {
					locked[cr.mu] = struct{}{}
					continue // leave the record locked
				}
			}
		}
		cr.mu.Unlock()
	}

	if gcMode && len(toDelete) > 0 {
		sortDeleteRecords(toDelete)
		var err error
		for i, cr := range toDelete {
			// only remove single record at a time
			if i == 0 {
				cr.dead = true
				err = setDeleted(cr.md)
			}
			cr.mu.Unlock()
		}
		if err != nil {
			return err
		}
		toDelete = toDelete[:1]
	}

	cm.mu.Unlock()

	if len(toDelete) == 0 {
		return nil
	}

	var err error
	for _, cr := range toDelete {
		cr.mu.Lock()

		usageCount, lastUsedAt := getLastUsed(cr.md)

		c := client.UsageInfo{
			ID:          cr.ID(),
			Mutable:     cr.mutable,
			InUse:       len(cr.refs) > 0,
			Size:        getSize(cr.md),
			CreatedAt:   GetCreatedAt(cr.md),
			Description: GetDescription(cr.md),
			LastUsedAt:  lastUsedAt,
			UsageCount:  usageCount,
		}

		if cr.parent != nil {
			c.Parent = cr.parent.ID()
		}
		if c.Size == sizeUnknown && cr.equalImmutable != nil {
			c.Size = getSize(cr.equalImmutable.md) // benefit from DiskUsage calc
		}
		if c.Size == sizeUnknown {
			cr.mu.Unlock() // all the non-prune modifications already protected by cr.dead
			s, err := cr.Size(ctx)
			if err != nil {
				return err
			}
			c.Size = s
			cr.mu.Lock()
		}

		opt.totalSize -= c.Size

		if cr.equalImmutable != nil {
			if err1 := cr.equalImmutable.remove(ctx, false); err == nil {
				err = err1
			}
		}
		if err1 := cr.remove(ctx, true); err == nil {
			err = err1
		}

		if err == nil && ch != nil {
			ch <- c
		}
		cr.mu.Unlock()
	}
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return cm.prune(ctx, ch, opt)
	}
}

func (cm *cacheManager) markShared(m map[string]*cacheUsageInfo) error {
	if cm.PruneRefChecker == nil {
		return nil
	}
	c, err := cm.PruneRefChecker()
	if err != nil {
		return errors.WithStack(err)
	}

	var markAllParentsShared func(string)
	markAllParentsShared = func(id string) {
		if v, ok := m[id]; ok {
			v.shared = true
			if v.parent != "" {
				markAllParentsShared(v.parent)
			}
		}
	}

	for id := range m {
		if m[id].shared {
			continue
		}
		if b := c.Exists(id, m[id].parentChain); b {
			markAllParentsShared(id)
		}
	}
	return nil
}

type cacheUsageInfo struct {
	refs        int
	parent      string
	size        int64
	mutable     bool
	createdAt   time.Time
	usageCount  int
	lastUsedAt  *time.Time
	description string
	doubleRef   bool
	recordType  client.UsageRecordType
	shared      bool
	parentChain []digest.Digest
}

func (cm *cacheManager) DiskUsage(ctx context.Context, opt client.DiskUsageInfo) ([]*client.UsageInfo, error) {
	filter, err := filters.ParseAll(opt.Filter...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse diskusage filters %v", opt.Filter)
	}

	cm.mu.Lock()

	m := make(map[string]*cacheUsageInfo, len(cm.records))
	rescan := make(map[string]struct{}, len(cm.records))

	for id, cr := range cm.records {
		cr.mu.Lock()
		// ignore duplicates that share data
		if cr.equalImmutable != nil && len(cr.equalImmutable.refs) > 0 || cr.equalMutable != nil && len(cr.refs) == 0 {
			cr.mu.Unlock()
			continue
		}

		usageCount, lastUsedAt := getLastUsed(cr.md)
		c := &cacheUsageInfo{
			refs:        len(cr.refs),
			mutable:     cr.mutable,
			size:        getSize(cr.md),
			createdAt:   GetCreatedAt(cr.md),
			usageCount:  usageCount,
			lastUsedAt:  lastUsedAt,
			description: GetDescription(cr.md),
			doubleRef:   cr.equalImmutable != nil,
			recordType:  GetRecordType(cr),
			parentChain: cr.parentChain(),
		}
		if c.recordType == "" {
			c.recordType = client.UsageRecordTypeRegular
		}
		if cr.parent != nil {
			c.parent = cr.parent.ID()
		}
		if cr.mutable && c.refs > 0 {
			c.size = 0 // size can not be determined because it is changing
		}
		m[id] = c
		rescan[id] = struct{}{}
		cr.mu.Unlock()
	}
	cm.mu.Unlock()

	for {
		if len(rescan) == 0 {
			break
		}
		for id := range rescan {
			v := m[id]
			if v.refs == 0 && v.parent != "" {
				m[v.parent].refs--
				if v.doubleRef {
					m[v.parent].refs--
				}
				rescan[v.parent] = struct{}{}
			}
			delete(rescan, id)
		}
	}

	if err := cm.markShared(m); err != nil {
		return nil, err
	}

	var du []*client.UsageInfo
	for id, cr := range m {
		c := &client.UsageInfo{
			ID:          id,
			Mutable:     cr.mutable,
			InUse:       cr.refs > 0,
			Size:        cr.size,
			Parent:      cr.parent,
			CreatedAt:   cr.createdAt,
			Description: cr.description,
			LastUsedAt:  cr.lastUsedAt,
			UsageCount:  cr.usageCount,
			RecordType:  cr.recordType,
			Shared:      cr.shared,
		}
		if filter.Match(adaptUsageInfo(c)) {
			du = append(du, c)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	for _, d := range du {
		if d.Size == sizeUnknown {
			func(d *client.UsageInfo) {
				eg.Go(func() error {
					ref, err := cm.Get(ctx, d.ID, NoUpdateLastUsed)
					if err != nil {
						d.Size = 0
						return nil
					}
					s, err := ref.Size(ctx)
					if err != nil {
						return err
					}
					d.Size = s
					return ref.Release(context.TODO())
				})
			}(d)
		}
	}

	if err := eg.Wait(); err != nil {
		return du, err
	}

	return du, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, errNotFound)
}

type RefOption interface{}

type cachePolicy int

const (
	cachePolicyDefault cachePolicy = iota
	cachePolicyRetain
)

type withMetadata interface {
	Metadata() *metadata.StorageItem
}

type noUpdateLastUsed struct{}

var NoUpdateLastUsed noUpdateLastUsed

func HasCachePolicyRetain(m withMetadata) bool {
	return getCachePolicy(m.Metadata()) == cachePolicyRetain
}

func CachePolicyRetain(m withMetadata) error {
	return queueCachePolicy(m.Metadata(), cachePolicyRetain)
}

func CachePolicyDefault(m withMetadata) error {
	return queueCachePolicy(m.Metadata(), cachePolicyDefault)
}

func WithDescription(descr string) RefOption {
	return func(m withMetadata) error {
		return queueDescription(m.Metadata(), descr)
	}
}

func WithRecordType(t client.UsageRecordType) RefOption {
	return func(m withMetadata) error {
		return queueRecordType(m.Metadata(), t)
	}
}

func WithCreationTime(tm time.Time) RefOption {
	return func(m withMetadata) error {
		return queueCreatedAt(m.Metadata(), tm)
	}
}

// Need a separate type for imageRef because it needs to be called outside
// initializeMetadata while still being a RefOption, so wrapping it in a
// different type ensures initializeMetadata won't catch it too and duplicate
// setting the metadata.
type imageRefOption func(m withMetadata) error

// WithImageRef appends the given imageRef to the cache ref's metadata
func WithImageRef(imageRef string) RefOption {
	return imageRefOption(func(m withMetadata) error {
		return appendImageRef(m.Metadata(), imageRef)
	})
}

func setImageRefMetadata(m withMetadata, opts ...RefOption) error {
	md := m.Metadata()
	for _, opt := range opts {
		if fn, ok := opt.(imageRefOption); ok {
			if err := fn(m); err != nil {
				return err
			}
		}
	}
	return md.Commit()
}

func initializeMetadata(m withMetadata, parent string, opts ...RefOption) error {
	md := m.Metadata()
	if tm := GetCreatedAt(md); !tm.IsZero() {
		return nil
	}

	if err := queueParent(md, parent); err != nil {
		return err
	}

	if err := queueCreatedAt(md, time.Now()); err != nil {
		return err
	}

	for _, opt := range opts {
		if fn, ok := opt.(func(withMetadata) error); ok {
			if err := fn(m); err != nil {
				return err
			}
		}
	}

	return md.Commit()
}

func adaptUsageInfo(info *client.UsageInfo) filters.Adaptor {
	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		if len(fieldpath) == 0 {
			return "", false
		}

		switch fieldpath[0] {
		case "id":
			return info.ID, info.ID != ""
		case "parent":
			return info.Parent, info.Parent != ""
		case "description":
			return info.Description, info.Description != ""
		case "inuse":
			return "", info.InUse
		case "mutable":
			return "", info.Mutable
		case "immutable":
			return "", !info.Mutable
		case "type":
			return string(info.RecordType), info.RecordType != ""
		case "shared":
			return "", info.Shared
		case "private":
			return "", !info.Shared
		}

		// TODO: add int/datetime/bytes support for more fields

		return "", false
	})
}

type pruneOpt struct {
	filter       filters.Filter
	all          bool
	checkShared  ExternalRefChecker
	keepDuration time.Duration
	keepBytes    int64
	totalSize    int64
}

type deleteRecord struct {
	*cacheRecord
	lastUsedAt      *time.Time
	usageCount      int
	lastUsedAtIndex int
	usageCountIndex int
}

func sortDeleteRecords(toDelete []*deleteRecord) {
	sort.Slice(toDelete, func(i, j int) bool {
		if toDelete[i].lastUsedAt == nil {
			return true
		}
		if toDelete[j].lastUsedAt == nil {
			return false
		}
		return toDelete[i].lastUsedAt.Before(*toDelete[j].lastUsedAt)
	})

	maxLastUsedIndex := 0
	var val time.Time
	for _, v := range toDelete {
		if v.lastUsedAt != nil && v.lastUsedAt.After(val) {
			val = *v.lastUsedAt
			maxLastUsedIndex++
		}
		v.lastUsedAtIndex = maxLastUsedIndex
	}

	sort.Slice(toDelete, func(i, j int) bool {
		return toDelete[i].usageCount < toDelete[j].usageCount
	})

	maxUsageCountIndex := 0
	var count int
	for _, v := range toDelete {
		if v.usageCount != count {
			count = v.usageCount
			maxUsageCountIndex++
		}
		v.usageCountIndex = maxUsageCountIndex
	}

	sort.Slice(toDelete, func(i, j int) bool {
		return float64(toDelete[i].lastUsedAtIndex)/float64(maxLastUsedIndex)+
			float64(toDelete[i].usageCountIndex)/float64(maxUsageCountIndex) <
			float64(toDelete[j].lastUsedAtIndex)/float64(maxLastUsedIndex)+
				float64(toDelete[j].usageCountIndex)/float64(maxUsageCountIndex)
	})
}

func diffIDFromDescriptor(desc ocispec.Descriptor) (digest.Digest, error) {
	diffIDStr, ok := desc.Annotations["containerd.io/uncompressed"]
	if !ok {
		return "", errors.Errorf("missing uncompressed annotation for %s", desc.Digest)
	}
	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse diffID %q for %s", diffIDStr, desc.Digest)
	}
	return diffID, nil
}
