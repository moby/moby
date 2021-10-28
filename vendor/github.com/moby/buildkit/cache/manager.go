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
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
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
	ContentStore    content.Store
	LeaseManager    leases.Manager
	PruneRefChecker ExternalRefCheckerFunc
	GarbageCollect  func(ctx context.Context) (gc.Stats, error)
	Applier         diff.Applier
	Differ          diff.Comparer
	MetadataStore   *metadata.Store
}

type Accessor interface {
	MetadataStore

	GetByBlob(ctx context.Context, desc ocispecs.Descriptor, parent ImmutableRef, opts ...RefOption) (ImmutableRef, error)
	Get(ctx context.Context, id string, opts ...RefOption) (ImmutableRef, error)

	New(ctx context.Context, parent ImmutableRef, s session.Group, opts ...RefOption) (MutableRef, error)
	GetMutable(ctx context.Context, id string, opts ...RefOption) (MutableRef, error) // Rebase?
	IdentityMapping() *idtools.IdentityMapping
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

	muPrune sync.Mutex // make sure parallel prune is not allowed so there will not be inconsistent results
	unlazyG flightcontrol.Group
}

func NewManager(opt ManagerOpt) (Manager, error) {
	cm := &cacheManager{
		ManagerOpt: opt,
		records:    make(map[string]*cacheRecord),
	}

	if err := cm.init(context.TODO()); err != nil {
		return nil, err
	}

	// cm.scheduleGC(5 * time.Minute)

	return cm, nil
}

func (cm *cacheManager) GetByBlob(ctx context.Context, desc ocispecs.Descriptor, parent ImmutableRef, opts ...RefOption) (ir ImmutableRef, rerr error) {
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
		p2, err := cm.Get(ctx, parent.ID(), NoUpdateLastUsed, descHandlers)
		if err != nil {
			return nil, err
		}
		p = p2.(*immutableRef)

		if p.getChainID() == "" || p.getBlobChainID() == "" {
			p.Release(context.TODO())
			return nil, errors.Errorf("failed to get ref by blob on non-addressable parent")
		}
		chainID = imagespecidentity.ChainID([]digest.Digest{p.getChainID(), chainID})
		blobChainID = imagespecidentity.ChainID([]digest.Digest{p.getBlobChainID(), blobChainID})

		if err := p.Finalize(ctx); err != nil {
			p.Release(context.TODO())
			return nil, err
		}
		parentID = p.ID()
	}

	releaseParent := false
	defer func() {
		if releaseParent || rerr != nil && p != nil {
			p.Release(context.TODO())
		}
	}()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	sis, err := cm.searchBlobchain(ctx, blobChainID)
	if err != nil {
		return nil, err
	}

	for _, si := range sis {
		ref, err := cm.get(ctx, si.ID(), opts...)
		if err != nil && !IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to get record %s by blobchainid", sis[0].ID())
		}
		if ref == nil {
			continue
		}
		if p != nil {
			releaseParent = true
		}
		if err := setImageRefMetadata(ref.cacheMetadata, opts...); err != nil {
			return nil, errors.Wrapf(err, "failed to append image ref metadata to ref %s", ref.ID())
		}
		return ref, nil
	}

	sis, err = cm.searchChain(ctx, chainID)
	if err != nil {
		return nil, err
	}

	var link *immutableRef
	for _, si := range sis {
		ref, err := cm.get(ctx, si.ID(), opts...)
		// if the error was NotFound or NeedsRemoteProvider, we can't re-use the snapshot from the blob so just skip it
		if err != nil && !IsNotFound(err) && !errors.As(err, &NeedsRemoteProvidersError{}) {
			return nil, errors.Wrapf(err, "failed to get record %s by chainid", si.ID())
		}
		if ref != nil {
			link = ref
			break
		}
	}

	id := identity.NewID()
	snapshotID := chainID.String()
	blobOnly := true
	if link != nil {
		snapshotID = link.getSnapshotID()
		blobOnly = link.getBlobOnly()
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

	md, _ := cm.getMetadata(id)

	rec := &cacheRecord{
		mu:            &sync.Mutex{},
		cm:            cm,
		refs:          make(map[ref]struct{}),
		parent:        p,
		cacheMetadata: md,
	}

	if err := initializeMetadata(rec.cacheMetadata, parentID, opts...); err != nil {
		return nil, err
	}

	if err := setImageRefMetadata(rec.cacheMetadata, opts...); err != nil {
		return nil, errors.Wrapf(err, "failed to append image ref metadata to ref %s", rec.ID())
	}

	rec.queueDiffID(diffID)
	rec.queueBlob(desc.Digest)
	rec.queueChainID(chainID)
	rec.queueBlobChainID(blobChainID)
	rec.queueSnapshotID(snapshotID)
	rec.queueBlobOnly(blobOnly)
	rec.queueMediaType(desc.MediaType)
	rec.queueBlobSize(desc.Size)
	rec.queueCommitted(true)

	if err := rec.commitMetadata(); err != nil {
		return nil, err
	}

	cm.records[id] = rec

	return rec.ref(true, descHandlers), nil
}

// init loads all snapshots from metadata state and tries to load the records
// from the snapshotter. If snaphot can't be found, metadata is deleted as well.
func (cm *cacheManager) init(ctx context.Context) error {
	items, err := cm.MetadataStore.All()
	if err != nil {
		return err
	}

	for _, si := range items {
		if _, err := cm.getRecord(ctx, si.ID()); err != nil {
			logrus.Debugf("could not load snapshot %s: %+v", si.ID(), err)
			cm.MetadataStore.Clear(si.ID())
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
	return cm.MetadataStore.Close()
}

// Get returns an immutable snapshot reference for ID
func (cm *cacheManager) Get(ctx context.Context, id string, opts ...RefOption) (ImmutableRef, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.get(ctx, id, opts...)
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
			blob := rec.getBlob()
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

	md, ok := cm.getMetadata(id)
	if !ok {
		return nil, errors.Wrap(errNotFound, id)
	}
	if mutableID := md.getEqualMutable(); mutableID != "" {
		mutable, err := cm.getRecord(ctx, mutableID)
		if err != nil {
			// check loading mutable deleted record from disk
			if IsNotFound(err) {
				cm.MetadataStore.Clear(id)
			}
			return nil, err
		}

		// parent refs are possibly lazy so keep it hold the description handlers.
		var dhs DescHandlers
		if mutable.parent != nil {
			dhs = mutable.parent.descHandlers
		}
		rec := &cacheRecord{
			mu:            &sync.Mutex{},
			cm:            cm,
			refs:          make(map[ref]struct{}),
			parent:        mutable.parentRef(false, dhs),
			cacheMetadata: md,
			equalMutable:  &mutableRef{cacheRecord: mutable},
		}
		mutable.equalImmutable = &immutableRef{cacheRecord: rec}
		cm.records[id] = rec
		return rec, nil
	}

	var parent *immutableRef
	if parentID := md.getParent(); parentID != "" {
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
		mu:            &sync.Mutex{},
		mutable:       !md.getCommitted(),
		cm:            cm,
		refs:          make(map[ref]struct{}),
		parent:        parent,
		cacheMetadata: md,
	}

	// the record was deleted but we crashed before data on disk was removed
	if md.getDeleted() {
		if err := rec.remove(ctx, true); err != nil {
			return nil, err
		}
		return nil, errors.Wrapf(errNotFound, "failed to get deleted record %s", id)
	}

	if err := initializeMetadata(rec.cacheMetadata, md.getParent(), opts...); err != nil {
		return nil, err
	}

	if err := setImageRefMetadata(rec.cacheMetadata, opts...); err != nil {
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
		if err := parent.Finalize(ctx); err != nil {
			return nil, err
		}
		if err := parent.Extract(ctx, sess); err != nil {
			return nil, err
		}
		parentSnapshotID = parent.getSnapshotID()
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

	if cm.Snapshotter.Name() == "stargz" && parent != nil {
		if rerr := parent.withRemoteSnapshotLabelsStargzMode(ctx, sess, func() {
			err = cm.Snapshotter.Prepare(ctx, id, parentSnapshotID)
		}); rerr != nil {
			return nil, rerr
		}
	} else {
		err = cm.Snapshotter.Prepare(ctx, id, parentSnapshotID)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare %s", id)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	md, _ := cm.getMetadata(id)

	rec := &cacheRecord{
		mu:            &sync.Mutex{},
		mutable:       true,
		cm:            cm,
		refs:          make(map[ref]struct{}),
		parent:        parent,
		cacheMetadata: md,
	}

	opts = append(opts, withSnapshotID(id))
	if err := initializeMetadata(rec.cacheMetadata, parentID, opts...); err != nil {
		return nil, err
	}

	if err := setImageRefMetadata(rec.cacheMetadata, opts...); err != nil {
		return nil, errors.Wrapf(err, "failed to append image ref metadata to ref %s", rec.ID())
	}

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
			recordType := cr.GetRecordType()
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

			usageCount, lastUsedAt := cr.getLastUsed()
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
					if err := cr.queueDeleted(); err != nil {
						cr.mu.Unlock()
						cm.mu.Unlock()
						return err
					}
					if err := cr.commitMetadata(); err != nil {
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
				err = cr.queueDeleted()
				if err == nil {
					err = cr.commitMetadata()
				}
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

	// calculate sizes here so that lock does not need to be held for slow process
	for _, cr := range toDelete {
		size := cr.getSize()

		if size == sizeUnknown && cr.equalImmutable != nil {
			size = cr.equalImmutable.getSize() // benefit from DiskUsage calc
		}
		if size == sizeUnknown {
			// calling size will warm cache for next call
			if _, err := cr.size(ctx); err != nil {
				return err
			}
		}
	}

	cm.mu.Lock()
	var err error
	for _, cr := range toDelete {
		cr.mu.Lock()

		usageCount, lastUsedAt := cr.getLastUsed()

		c := client.UsageInfo{
			ID:          cr.ID(),
			Mutable:     cr.mutable,
			InUse:       len(cr.refs) > 0,
			Size:        cr.getSize(),
			CreatedAt:   cr.GetCreatedAt(),
			Description: cr.GetDescription(),
			LastUsedAt:  lastUsedAt,
			UsageCount:  usageCount,
		}

		if cr.parent != nil {
			c.Parent = cr.parent.ID()
		}
		if c.Size == sizeUnknown && cr.equalImmutable != nil {
			c.Size = cr.equalImmutable.getSize() // benefit from DiskUsage calc
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
	cm.mu.Unlock()
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

		usageCount, lastUsedAt := cr.getLastUsed()
		c := &cacheUsageInfo{
			refs:        len(cr.refs),
			mutable:     cr.mutable,
			size:        cr.getSize(),
			createdAt:   cr.GetCreatedAt(),
			usageCount:  usageCount,
			lastUsedAt:  lastUsedAt,
			description: cr.GetDescription(),
			doubleRef:   cr.equalImmutable != nil,
			recordType:  cr.GetRecordType(),
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
					cm.mu.Lock()
					ref, err := cm.get(ctx, d.ID, NoUpdateLastUsed)
					cm.mu.Unlock()
					if err != nil {
						d.Size = 0
						return nil
					}
					s, err := ref.size(ctx)
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

type noUpdateLastUsed struct{}

var NoUpdateLastUsed noUpdateLastUsed

func CachePolicyRetain(m *cacheMetadata) error {
	return m.SetCachePolicyRetain()
}

func CachePolicyDefault(m *cacheMetadata) error {
	return m.SetCachePolicyDefault()
}

func WithDescription(descr string) RefOption {
	return func(m *cacheMetadata) error {
		return m.queueDescription(descr)
	}
}

func WithRecordType(t client.UsageRecordType) RefOption {
	return func(m *cacheMetadata) error {
		return m.queueRecordType(t)
	}
}

func WithCreationTime(tm time.Time) RefOption {
	return func(m *cacheMetadata) error {
		return m.queueCreatedAt(tm)
	}
}

// Need a separate type for imageRef because it needs to be called outside
// initializeMetadata while still being a RefOption, so wrapping it in a
// different type ensures initializeMetadata won't catch it too and duplicate
// setting the metadata.
type imageRefOption func(m *cacheMetadata) error

// WithImageRef appends the given imageRef to the cache ref's metadata
func WithImageRef(imageRef string) RefOption {
	return imageRefOption(func(m *cacheMetadata) error {
		return m.appendImageRef(imageRef)
	})
}

func setImageRefMetadata(m *cacheMetadata, opts ...RefOption) error {
	for _, opt := range opts {
		if fn, ok := opt.(imageRefOption); ok {
			if err := fn(m); err != nil {
				return err
			}
		}
	}
	return m.commitMetadata()
}

func withSnapshotID(id string) RefOption {
	return imageRefOption(func(m *cacheMetadata) error {
		return m.queueSnapshotID(id)
	})
}

func initializeMetadata(m *cacheMetadata, parent string, opts ...RefOption) error {
	if tm := m.GetCreatedAt(); !tm.IsZero() {
		return nil
	}

	if err := m.queueParent(parent); err != nil {
		return err
	}

	if err := m.queueCreatedAt(time.Now()); err != nil {
		return err
	}

	for _, opt := range opts {
		if fn, ok := opt.(func(*cacheMetadata) error); ok {
			if err := fn(m); err != nil {
				return err
			}
		}
	}

	return m.commitMetadata()
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

func diffIDFromDescriptor(desc ocispecs.Descriptor) (digest.Digest, error) {
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
