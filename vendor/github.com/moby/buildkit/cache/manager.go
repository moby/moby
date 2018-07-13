package cache

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/snapshots"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/snapshot"
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
	Snapshotter   snapshot.SnapshotterBase
	GCPolicy      GCPolicy
	MetadataStore *metadata.Store
}

type Accessor interface {
	Get(ctx context.Context, id string, opts ...RefOption) (ImmutableRef, error)
	GetFromSnapshotter(ctx context.Context, id string, opts ...RefOption) (ImmutableRef, error)
	New(ctx context.Context, s ImmutableRef, opts ...RefOption) (MutableRef, error)
	GetMutable(ctx context.Context, id string) (MutableRef, error) // Rebase?
}

type Controller interface {
	DiskUsage(ctx context.Context, info client.DiskUsageInfo) ([]*client.UsageInfo, error)
	Prune(ctx context.Context, ch chan client.UsageInfo) error
	GC(ctx context.Context) error
}

type Manager interface {
	Accessor
	Controller
	Close() error
}

type cacheManager struct {
	records map[string]*cacheRecord
	mu      sync.Mutex
	ManagerOpt
	md *metadata.Store

	muPrune sync.Mutex // make sure parallel prune is not allowed so there will not be inconsistent results
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

// init loads all snapshots from metadata state and tries to load the records
// from the snapshotter. If snaphot can't be found, metadata is deleted as well.
func (cm *cacheManager) init(ctx context.Context) error {
	items, err := cm.md.All()
	if err != nil {
		return err
	}

	for _, si := range items {
		if _, err := cm.getRecord(ctx, si.ID(), false); err != nil {
			logrus.Debugf("could not load snapshot %s: %v", si.ID(), err)
			cm.md.Clear(si.ID())
			// TODO: make sure content is deleted as well
		}
	}
	return nil
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
	return cm.get(ctx, id, false, opts...)
}

// Get returns an immutable snapshot reference for ID
func (cm *cacheManager) GetFromSnapshotter(ctx context.Context, id string, opts ...RefOption) (ImmutableRef, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.get(ctx, id, true, opts...)
}

// get requires manager lock to be taken
func (cm *cacheManager) get(ctx context.Context, id string, fromSnapshotter bool, opts ...RefOption) (ImmutableRef, error) {
	rec, err := cm.getRecord(ctx, id, fromSnapshotter, opts...)
	if err != nil {
		return nil, err
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if rec.mutable {
		if len(rec.refs) != 0 {
			return nil, errors.Wrapf(ErrLocked, "%s is locked", id)
		}
		if rec.equalImmutable != nil {
			return rec.equalImmutable.ref(), nil
		}
		return rec.mref().commit(ctx)
	}

	return rec.ref(), nil
}

// getRecord returns record for id. Requires manager lock.
func (cm *cacheManager) getRecord(ctx context.Context, id string, fromSnapshotter bool, opts ...RefOption) (cr *cacheRecord, retErr error) {
	if rec, ok := cm.records[id]; ok {
		if rec.isDead() {
			return nil, errNotFound
		}
		return rec, nil
	}

	md, ok := cm.md.Get(id)
	if !ok && !fromSnapshotter {
		return nil, errNotFound
	}
	if mutableID := getEqualMutable(md); mutableID != "" {
		mutable, err := cm.getRecord(ctx, mutableID, fromSnapshotter)
		if err != nil {
			// check loading mutable deleted record from disk
			if errors.Cause(err) == errNotFound {
				cm.md.Clear(id)
			}
			return nil, err
		}
		rec := &cacheRecord{
			mu:           &sync.Mutex{},
			cm:           cm,
			refs:         make(map[Mountable]struct{}),
			parent:       mutable.Parent(),
			md:           md,
			equalMutable: &mutableRef{cacheRecord: mutable},
		}
		mutable.equalImmutable = &immutableRef{cacheRecord: rec}
		cm.records[id] = rec
		return rec, nil
	}

	info, err := cm.Snapshotter.Stat(ctx, id)
	if err != nil {
		return nil, errors.Wrap(errNotFound, err.Error())
	}

	var parent ImmutableRef
	if info.Parent != "" {
		parent, err = cm.get(ctx, info.Parent, fromSnapshotter, opts...)
		if err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				parent.Release(context.TODO())
			}
		}()
	}

	rec := &cacheRecord{
		mu:      &sync.Mutex{},
		mutable: info.Kind != snapshots.KindCommitted,
		cm:      cm,
		refs:    make(map[Mountable]struct{}),
		parent:  parent,
		md:      md,
	}

	// the record was deleted but we crashed before data on disk was removed
	if getDeleted(md) {
		if err := rec.remove(ctx, true); err != nil {
			return nil, err
		}
		return nil, errNotFound
	}

	if err := initializeMetadata(rec, opts...); err != nil {
		if parent != nil {
			parent.Release(context.TODO())
		}
		return nil, err
	}

	cm.records[id] = rec
	return rec, nil
}

func (cm *cacheManager) New(ctx context.Context, s ImmutableRef, opts ...RefOption) (MutableRef, error) {
	id := identity.NewID()

	var parent ImmutableRef
	var parentID string
	if s != nil {
		var err error
		parent, err = cm.Get(ctx, s.ID())
		if err != nil {
			return nil, err
		}
		if err := parent.Finalize(ctx); err != nil {
			return nil, err
		}
		parentID = parent.ID()
	}

	if err := cm.Snapshotter.Prepare(ctx, id, parentID); err != nil {
		if parent != nil {
			parent.Release(context.TODO())
		}
		return nil, errors.Wrapf(err, "failed to prepare %s", id)
	}

	md, _ := cm.md.Get(id)

	rec := &cacheRecord{
		mu:      &sync.Mutex{},
		mutable: true,
		cm:      cm,
		refs:    make(map[Mountable]struct{}),
		parent:  parent,
		md:      md,
	}

	if err := initializeMetadata(rec, opts...); err != nil {
		if parent != nil {
			parent.Release(context.TODO())
		}
		return nil, err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.records[id] = rec // TODO: save to db

	return rec.mref(), nil
}
func (cm *cacheManager) GetMutable(ctx context.Context, id string) (MutableRef, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	rec, err := cm.getRecord(ctx, id, false)
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

	return rec.mref(), nil
}

func (cm *cacheManager) Prune(ctx context.Context, ch chan client.UsageInfo) error {
	cm.muPrune.Lock()
	defer cm.muPrune.Unlock()
	return cm.prune(ctx, ch)
}

func (cm *cacheManager) prune(ctx context.Context, ch chan client.UsageInfo) error {
	var toDelete []*cacheRecord
	cm.mu.Lock()

	for _, cr := range cm.records {
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
			cr.dead = true
			toDelete = append(toDelete, cr)
		}

		// mark metadata as deleted in case we crash before cleanup finished
		if err := setDeleted(cr.md); err != nil {
			cr.mu.Unlock()
			cm.mu.Unlock()
			return err
		}
		cr.mu.Unlock()
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

		if c.Size == sizeUnknown {
			cr.mu.Unlock() // all the non-prune modifications already protected by cr.dead
			s, err := cr.Size(ctx)
			if err != nil {
				return err
			}
			c.Size = s
			cr.mu.Lock()
		}

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
		return cm.prune(ctx, ch)
	}
}

func (cm *cacheManager) DiskUsage(ctx context.Context, opt client.DiskUsageInfo) ([]*client.UsageInfo, error) {
	cm.mu.Lock()

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
	}

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

	var du []*client.UsageInfo
	for id, cr := range m {
		if opt.Filter != "" && !strings.HasPrefix(id, opt.Filter) {
			continue
		}

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
		}
		du = append(du, c)
	}

	eg, ctx := errgroup.WithContext(ctx)

	for _, d := range du {
		if d.Size == sizeUnknown {
			func(d *client.UsageInfo) {
				eg.Go(func() error {
					ref, err := cm.Get(ctx, d.ID)
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

func IsLocked(err error) bool {
	return errors.Cause(err) == ErrLocked
}

func IsNotFound(err error) bool {
	return errors.Cause(err) == errNotFound
}

type RefOption func(withMetadata) error

type cachePolicy int

const (
	cachePolicyDefault cachePolicy = iota
	cachePolicyRetain
)

type withMetadata interface {
	Metadata() *metadata.StorageItem
}

func HasCachePolicyRetain(m withMetadata) bool {
	return getCachePolicy(m.Metadata()) == cachePolicyRetain
}

func CachePolicyRetain(m withMetadata) error {
	return queueCachePolicy(m.Metadata(), cachePolicyRetain)
}

func WithDescription(descr string) RefOption {
	return func(m withMetadata) error {
		return queueDescription(m.Metadata(), descr)
	}
}

func WithCreationTime(tm time.Time) RefOption {
	return func(m withMetadata) error {
		return queueCreatedAt(m.Metadata(), tm)
	}
}

func initializeMetadata(m withMetadata, opts ...RefOption) error {
	md := m.Metadata()
	if tm := GetCreatedAt(md); !tm.IsZero() {
		return nil
	}

	if err := queueCreatedAt(md, time.Now()); err != nil {
		return err
	}

	for _, opt := range opts {
		if err := opt(m); err != nil {
			return err
		}
	}

	return md.Commit()
}
