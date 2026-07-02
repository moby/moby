/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package manager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"

	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/metadata"
	"github.com/containerd/containerd/v2/core/metadata/boltutil"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/pkg/gc"
	"github.com/containerd/containerd/v2/pkg/namespaces"
)

type BoltManager interface {
	mount.Manager
	metadata.Collector
	Sync(context.Context) error
}

type managerOptions struct {
	handlers map[string]mount.Handler
	roots    []*os.Root
}

type Opt func(*managerOptions) error

func WithMountHandler(name string, h mount.Handler) Opt {
	return func(o *managerOptions) error {
		if o.handlers == nil {
			o.handlers = make(map[string]mount.Handler)
		}
		o.handlers[name] = h
		return nil
	}
}

func WithAllowedRoot(root string) Opt {
	return func(o *managerOptions) error {
		r, err := os.OpenRoot(root)
		if err != nil {
			return err
		}
		o.roots = append(o.roots, r)
		return nil
	}
}

func NewManager(db *bolt.DB, targetDir string, opts ...Opt) (mount.Manager, error) {
	options := managerOptions{}
	for _, o := range opts {
		if err := o(&options); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return nil, err
	}
	tr, err := os.OpenRoot(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open target root %q: %w", targetDir, err)
	}
	rootMap := map[string]*os.Root{
		tr.Name(): tr,
	}
	for _, r := range options.roots {
		rootMap[r.Name()] = r
	}

	return &mountManager{
		db:       db,
		targets:  tr,
		handlers: options.handlers,
		rootMap:  rootMap,
	}, nil
}

type mountManager struct {
	db       *bolt.DB
	targets  *os.Root
	handlers map[string]mount.Handler
	rootMap  map[string]*os.Root

	rwlock sync.RWMutex
}

func (mm *mountManager) Close() error {
	var errs []error
	for _, r := range mm.rootMap {
		if err := r.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (mm *mountManager) Activate(ctx context.Context, name string, mounts []mount.Mount, opts ...mount.ActivateOpt) (info mount.ActivationInfo, retErr error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return mount.ActivationInfo{}, err
	}

	log.G(ctx).WithField("name", name).WithField("mounts", mounts).Debugf("activating mount")

	lid, leased := leases.FromContext(ctx)

	var config mount.ActivateOptions
	for _, opt := range opts {
		opt(&config)
	}

	shouldTransform := func(p string, t string) bool {
		p = p + "/*"
		for _, mt := range config.AllowMountTypes {
			if mt == p || mt == t {
				return false
			}
		}
		return true
	}

	shouldHandle := func(t string) bool {
		return !slices.Contains(config.AllowMountTypes, t)
	}

	transforms := map[string]mount.Transformer{
		"format": mountFormatter{},
		"mkfs": &mkfs{
			rootMap: mm.rootMap,
		},
		"mkdir": &mkdir{
			rootMap: mm.rootMap,
		},
	}

	start := time.Now()
	// highest index of a mount
	// first system mount is the first index which should be mounted by the system
	var firstSystemMount = -1
	var mountConv [][]mount.Transformer
	var handlers []mount.Handler
	for i := range mounts {
		mountType := mounts[i].Type

		// Check is the source needs transformation, any transform operation requires
		// mounting with the mount manager.
		for transformType, mt, ok := strings.Cut(mountType, "/"); ok; transformType, mt, ok = strings.Cut(mountType, "/") {
			if tr, ok := transforms[transformType]; ok {
				if shouldTransform(transformType, mounts[i].Type) {
					// At least everything before this must be mounted
					// by the mount manager
					firstSystemMount = i
				}

				if handlers == nil {
					handlers = make([]mount.Handler, len(mounts))
				}

				if mountConv == nil {
					mountConv = make([][]mount.Transformer, len(mounts))
				}

				mountConv[i] = append(mountConv[i], typeTransformer{
					Transformer: tr,
					mountType:   mt,
				})

				mountType = mt
			} else {
				log.G(ctx).Warnf("unknown transform %q for mount %v", transformType, mounts[i])
				break
			}
		}

		var handler mount.Handler
		if mm.handlers != nil {
			handler = mm.handlers[mountType]
		}

		if handler != nil || config.Temporary {
			if handlers == nil {
				handlers = make([]mount.Handler, len(mounts))
			}
			handlers[i] = handler
			if shouldHandle(mountType) || config.Temporary {
				firstSystemMount = i + 1
			}
		}
	}
	// If no mounts are handled here, return not implemented and caller
	// may just perform system mounts as normal.
	if firstSystemMount == -1 {
		return mount.ActivationInfo{}, errdefs.ErrNotImplemented
	}

	// Get read lock to block GC context from starting
	mm.rwlock.RLock()
	defer mm.rwlock.RUnlock()

	var mid uint64

	if err := mm.db.Update(func(tx *bolt.Tx) error {
		v1bkt, err := tx.CreateBucketIfNotExists([]byte("v1"))
		if err != nil {
			return err
		}

		nsbkt, err := v1bkt.CreateBucketIfNotExists([]byte(namespace))
		if err != nil {
			return err
		}
		mbkt, err := nsbkt.CreateBucketIfNotExists(bucketKeyMounts)
		if err != nil {
			return err
		}
		bkt, err := mbkt.CreateBucket([]byte(name))
		if err != nil {
			// If already exists, return already exists
			return err
		}

		mid, err = v1bkt.NextSequence()
		if err != nil {
			return err
		}

		idb, err := encodeID(mid)
		if err != nil {
			return err
		}
		if err = bkt.Put(bucketKeyID, idb); err != nil {
			return err
		}

		if err := boltutil.WriteLabels(bkt, config.Labels); err != nil {
			return err
		}

		if err := boltutil.WriteTimestamps(bkt, start, start); err != nil {
			return err
		}

		if leased {
			if err = bkt.Put(bucketKeyLease, []byte(lid)); err != nil {
				return err
			}

			lsbkt, err := nsbkt.CreateBucketIfNotExists(bucketKeyLeases)
			if err != nil {
				return err
			}
			lbkt, err := lsbkt.CreateBucketIfNotExists([]byte(lid))
			if err != nil {
				return err
			}
			if err := lbkt.Put([]byte(name), nil); err != nil {
				return err
			}
		}

		// TODO: Store mount information including mountpoint
		// Setup mounts now with generated targets

		return nil
	}); err != nil {
		return mount.ActivationInfo{}, err
	}

	defer func() {
		// If error, rollback and remove by name
		if retErr != nil {
			if err := mm.db.Update(func(tx *bolt.Tx) error {
				v1bkt := tx.Bucket([]byte("v1"))
				if v1bkt == nil {
					return fmt.Errorf("missing bucket: %w", errdefs.ErrUnknown)
				}

				nsbkt := v1bkt.Bucket([]byte(namespace))
				if nsbkt == nil {
					return fmt.Errorf("missing namespace %q bucket: %w", namespace, errdefs.ErrUnknown)
				}

				mbkt := nsbkt.Bucket(bucketKeyMounts)
				if mbkt == nil {
					return fmt.Errorf("missing mounts bucket: %w", errdefs.ErrUnknown)
				}

				if leased {
					lsbkt := nsbkt.Bucket(bucketKeyLeases)
					if lsbkt != nil {
						lbkt := lsbkt.Bucket([]byte(lid))
						if lbkt != nil {
							lbkt.Delete([]byte(name))
						}
						if k, _ := lbkt.Cursor().First(); k == nil {
							lsbkt.DeleteBucket([]byte(lid))
						}
					}

				}

				return mbkt.DeleteBucket([]byte(name))
			}); err != nil {
				log.G(ctx).WithError(err).WithField("name", name).Errorf("failed to rollback")
			}
		}
	}()

	targetName := strconv.FormatUint(mid, 10)
	if err := mm.targets.Mkdir(targetName, 0700); err != nil {
		return mount.ActivationInfo{}, err
	}

	var mounted []mount.ActiveMount
	defer func() {
		// If error, unmount all mounted
		if retErr != nil {
			for i, m := range mounted {
				var err error
				if h := handlers[i]; h != nil {
					err = h.Unmount(ctx, m.MountPoint)
				} else {
					err = mount.Unmount(m.MountPoint, 0)
				}
				if err != nil {
					log.G(ctx).WithError(err).WithField("MountPoint", m.MountPoint).Error("failed to cleanup mount after failed activation")
				}
			}
		}
	}()

	// Ensure directory order for cleanup when rare case of large number of mounts,
	// this allows cleanup logic to just scan directories on cleanup.
	formatMP := "%d"
	formatType := "%d-type"
	if firstSystemMount > 100 {
		formatMP = "%03d"
		formatType = "%03d-type"
	} else if firstSystemMount > 10 {
		formatMP = "%02d"
		formatType = "%02d-type"
	}

	for i, m := range mounts[:firstSystemMount] {
		if mountConv != nil && mountConv[i] != nil {
			for _, tr := range mountConv[i] {
				newM, err := tr.Transform(ctx, m, mounted)
				if err != nil {
					return mount.ActivationInfo{}, err
				}
				m = newM
			}
			mounts[i] = m
		}

		// Use cleanup order for directory names
		ci := firstSystemMount - i
		// TODO: Go 1.25 use targetbase.WriteFile
		if err := os.WriteFile(filepath.Join(mm.targets.Name(), targetName, fmt.Sprintf(formatType, ci)), []byte(m.Type), 0600); err != nil {
			return mount.ActivationInfo{}, err
		}

		mname := fmt.Sprintf(formatMP, ci)
		var active mount.ActiveMount
		if h := handlers[i]; h != nil {
			active, err = h.Mount(ctx, m, filepath.Join(mm.targets.Name(), targetName, mname), mounted)
			if err != nil {
				return mount.ActivationInfo{}, fmt.Errorf("mount handler failed %v: %w", m, err)
			}
		} else {
			if err := mm.targets.Mkdir(filepath.Join(targetName, mname), 0700); err != nil {
				return mount.ActivationInfo{}, err
			}
			mp := filepath.Join(mm.targets.Name(), targetName, mname)
			if err := m.Mount(mp); err != nil {
				return mount.ActivationInfo{}, fmt.Errorf("mount failed %v: %w", m, err)
			}
			t := time.Now()
			active = mount.ActiveMount{
				Mount:      m,
				MountPoint: mp,
				MountedAt:  &t,
			}
		}
		mounted = append(mounted, active)
	}

	// If first system mount is converted, fill in the format
	if mountConv != nil {
		for _, tr := range mountConv[firstSystemMount] {
			newM, err := tr.Transform(ctx, mounts[firstSystemMount], mounted)
			if err != nil {
				return mount.ActivationInfo{}, err
			}
			mounts[firstSystemMount] = newM
		}
	}
	// If no system mounts, add a bind mount if temporary
	// TODO: Add config for whether to add the bind mount?
	if config.Temporary && firstSystemMount > 0 {
		mounts = append(mounts, mount.Mount{
			Type:    "bind",
			Source:  mounted[firstSystemMount-1].MountPoint,
			Options: []string{"rbind"},
		})
	}

	info.Name = name
	info.Active = mounted
	info.System = mounts[firstSystemMount:]
	info.Labels = config.Labels

	// Open another write transaction and update state, or another way to update state?
	if err := mm.db.Update(func(tx *bolt.Tx) error {
		v1bkt := tx.Bucket([]byte("v1"))
		if v1bkt == nil {
			return fmt.Errorf("missing v1 bucket: %w", errdefs.ErrUnknown)
		}

		nsbkt := v1bkt.Bucket([]byte(namespace))
		if nsbkt == nil {
			return fmt.Errorf("missing namespace %q bucket: %w", namespace, errdefs.ErrUnknown)
		}

		mbkt := nsbkt.Bucket(bucketKeyMounts)
		if mbkt == nil {
			return fmt.Errorf("missing mounts bucket: %w", errdefs.ErrUnknown)
		}
		bkt := mbkt.Bucket([]byte(name))
		if bkt == nil {
			return fmt.Errorf("missing mount %q bucket: %w", name, errdefs.ErrUnknown)
		}

		abkt, err := bkt.CreateBucket(bucketKeyActive)
		if err != nil {
			return err
		}

		for i, active := range mounted {
			// Error is i > uint8 max
			cur, err := abkt.CreateBucket([]byte{byte(i)})
			if err != nil {
				return err
			}
			if err = putActiveMount(cur, active); err != nil {
				return err
			}

		}

		if err := boltutil.WriteTimestamps(bkt, start, time.Now()); err != nil {
			return err
		}

		// TODO: Save all system mounts

		return nil
	}); err != nil {
		return mount.ActivationInfo{}, err
	}

	return
}

func encodeID(id uint64) ([]byte, error) {
	var (
		buf       [binary.MaxVarintLen64]byte
		idEncoded = buf[:]
	)
	idEncoded = idEncoded[:binary.PutUvarint(idEncoded, id)]

	if len(idEncoded) == 0 {
		return nil, fmt.Errorf("failed encoding id = %v", id)
	}
	return idEncoded, nil
}

func readID(bkt *bolt.Bucket) uint64 {
	id, _ := binary.Uvarint(bkt.Get(bucketKeyID))
	return id
}

func putActiveMount(bkt *bolt.Bucket, active mount.ActiveMount) error {
	if err := bkt.Put(bucketKeyType, []byte(active.Type)); err != nil {
		return err
	}

	// TODO: Same if device?
	if err := bkt.Put(bucketKeyMountPoint, []byte(active.MountPoint)); err != nil {
		return err
	}

	mountedAt, err := active.MountedAt.MarshalBinary()
	if err != nil {
		return err
	}
	if err := bkt.Put(bucketKeyMountedAt, mountedAt); err != nil {
		return err
	}

	// TODO: Add Source
	// TODO: Add Target
	// TODO: Add Options

	return nil
}

func readActiveMount(bkt *bolt.Bucket) (mount.ActiveMount, error) {
	var active mount.ActiveMount
	active.Type = string(bkt.Get(bucketKeyType))
	active.MountPoint = string(bkt.Get(bucketKeyMountPoint))
	if v := bkt.Get(bucketKeyMountedAt); v != nil {
		var mountedAt time.Time
		if err := mountedAt.UnmarshalBinary(v); err != nil {
			// TODO: Should this be skipped or otherwise logged and ignored?
			return mount.ActiveMount{}, err
		}
		active.MountedAt = &mountedAt
	}

	return active, nil
}

func readActivationInfo(name string, bkt *bolt.Bucket) (mount.ActivationInfo, error) {
	info := mount.ActivationInfo{
		Name: name,
	}
	if abkt := bkt.Bucket(bucketKeyActive); abkt != nil {
		if err := abkt.ForEachBucket(func(k []byte) error {
			active, err := readActiveMount(abkt.Bucket(k))
			if err != nil {
				return err
			}
			info.Active = append(info.Active, active)
			return nil
		}); err != nil {
			return mount.ActivationInfo{}, err
		}
	}
	lbls, err := boltutil.ReadLabels(bkt)
	if err != nil {
		return mount.ActivationInfo{}, err
	}
	info.Labels = lbls

	return info, nil
}

func getBucket(tx *bolt.Tx, keys ...[]byte) *bolt.Bucket {
	bkt := tx.Bucket(keys[0])
	if bkt == nil {
		return nil
	}

	for _, key := range keys[1:] {
		bkt = bkt.Bucket(key)
		if bkt == nil {
			return nil
		}
	}

	return bkt
}

func (mm *mountManager) Deactivate(ctx context.Context, name string) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	var (
		mid       uint64
		allActive []mount.ActiveMount
	)

	// First in a single transaction, mark the mounts as deactivated
	if err := mm.db.Update(func(tx *bolt.Tx) error {
		v1bkt := tx.Bucket([]byte("v1"))
		if v1bkt == nil {
			return fmt.Errorf("missing v1 bucket: %w", errdefs.ErrNotFound)
		}

		nsbkt := v1bkt.Bucket([]byte(namespace))
		if nsbkt == nil {
			return fmt.Errorf("missing namespace %q bucket: %w", namespace, errdefs.ErrNotFound)
		}

		mbkt := nsbkt.Bucket(bucketKeyMounts)
		if mbkt == nil {
			return fmt.Errorf("missing mounts bucket: %w", errdefs.ErrNotFound)
		}
		bkt := mbkt.Bucket([]byte(name))
		if bkt == nil {
			return fmt.Errorf("missing mount %q bucket: %w", name, errdefs.ErrNotFound)
		}

		mid = readID(bkt)

		lid := bkt.Get(bucketKeyLease)
		if lid != nil {
			lssbkt := nsbkt.Bucket(bucketKeyLeases)
			if lssbkt != nil {
				lsbkt := lssbkt.Bucket(lid)
				if lsbkt != nil {
					if err = lsbkt.Delete([]byte(name)); err != nil {
						return err
					}
				}
			}
		}

		abkt := bkt.Bucket(bucketKeyActive)
		if abkt != nil {
			abkt.ForEachBucket(func(k []byte) error {
				active, err := readActiveMount(abkt.Bucket(k))
				if err != nil {
					return err
				}
				allActive = append(allActive, active)
				return nil
			})
		}

		if err = mbkt.DeleteBucket([]byte(name)); err != nil {
			return err
		}

		// TODO: Is unmountq really needed or just delete?

		return nil
	}); err != nil {
		return err
	}

	// TODO: Should this also be backgrounded, no much can do on failure to unmount
	var mountErrors error
	for i := len(allActive) - 1; i >= 0; i-- {
		var err error
		if h := mm.handlers[allActive[i].Type]; h != nil {
			err = h.Unmount(ctx, allActive[i].MountPoint)
		} else {
			err = mount.Unmount(allActive[i].MountPoint, 0)
		}
		if err != nil {
			mountErrors = errors.Join(mountErrors, err)
		}
	}
	if mountErrors != nil {
		// Don't try to cleanup, GC will need to do the rest
		return mountErrors
	}

	// Run in background, GC would handle leftovers?
	// Make configurable?
	// TODO: In go 1.25, use mm.targets.RemoveAll()
	if err := os.RemoveAll(filepath.Join(mm.targets.Name(), fmt.Sprintf("%d", mid))); err != nil {
		// TODO: Only log here, cleanup would have to occur later
		log.G(ctx).WithError(err).WithField("mountid", mid).Error("failed to cleanup mount target")
	}

	return nil
}

func (mm *mountManager) Info(ctx context.Context, name string) (mount.ActivationInfo, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return mount.ActivationInfo{}, err
	}
	var info mount.ActivationInfo
	if err := mm.db.View(func(tx *bolt.Tx) error {
		bkt := getBucket(tx, []byte("v1"), []byte(namespace), bucketKeyMounts, []byte(name))
		if bkt == nil {
			return fmt.Errorf("mount %q %w", name, errdefs.ErrNotFound)
		}
		var err error
		info, err = readActivationInfo(name, bkt)
		return err
	}); err != nil {
		return mount.ActivationInfo{}, err
	}
	return info, nil
}

func (mm *mountManager) Update(context.Context, mount.ActivationInfo, ...string) (mount.ActivationInfo, error) {
	return mount.ActivationInfo{}, errdefs.ErrNotImplemented
}

func (mm *mountManager) List(ctx context.Context, filters ...string) ([]mount.ActivationInfo, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	var infos []mount.ActivationInfo
	if err := mm.db.View(func(tx *bolt.Tx) error {
		mbkt := getBucket(tx, []byte("v1"), []byte(namespace), bucketKeyMounts)
		if mbkt == nil {
			return nil
		}

		return mbkt.ForEachBucket(func(k []byte) error {
			info, err := readActivationInfo(string(k), mbkt.Bucket(k))
			if err != nil {
				return err
			}
			infos = append(infos, info)
			return nil
		})
	}); err != nil {
		return nil, err
	}
	return infos, nil
}

func (mm *mountManager) StartCollection(ctx context.Context) (metadata.CollectionContext, error) {
	// lock now and collection will unlock on cancel or finish
	mm.rwlock.Lock()

	tx, err := mm.db.Begin(true)
	if err != nil {
		return nil, err
	}

	return &collectionContext{
		ctx:     ctx,
		tx:      tx,
		manager: mm,
		removed: map[string]map[string]struct{}{},
	}, nil
}

func (mm *mountManager) ReferenceLabel() string {
	return "mount"
}

type collectionContext struct {
	ctx     context.Context
	tx      *bolt.Tx
	manager *mountManager
	removed map[string]map[string]struct{}
}

func (cc *collectionContext) All(fn func(gc.Node)) {
	v1bkt := cc.tx.Bucket([]byte("v1"))
	if v1bkt == nil {
		return
	}
	nsc := v1bkt.Cursor()
	for nsk, nsv := nsc.First(); nsk != nil; nsk, nsv = nsc.Next() {
		if nsv != nil {
			continue
		}
		mntsbkt := v1bkt.Bucket(nsk).Bucket(bucketKeyMounts)
		if mntsbkt == nil {
			continue
		}
		mc := mntsbkt.Cursor()
		for mk, mv := mc.First(); mk != nil; mk, mv = mc.Next() {
			if mv != nil {
				continue
			}
			fn(gc.Node{
				Type:      metadata.ResourceMount,
				Namespace: string(nsk),
				Key:       string(mk),
			})
		}
	}
}

func gcnode(t gc.ResourceType, ns, key string) gc.Node {
	return gc.Node{
		Type:      t,
		Namespace: ns,
		Key:       key,
	}
}

func (cc *collectionContext) ActiveWithBackRefs(ns string, fn func(gc.Node), bref func(gc.Node, gc.Node)) {
	nsbkt := getBucket(cc.tx, []byte("v1"), []byte(ns), bucketKeyMounts)
	if nsbkt != nil {
		mc := nsbkt.Cursor()
		for mk, mv := mc.First(); mk != nil; mk, mv = mc.Next() {
			if mv != nil {
				continue
			}
			n := gcnode(metadata.ResourceMount, ns, string(mk))
			lbkt := nsbkt.Bucket(mk).Bucket(bucketKeyLabels)
			if lbkt != nil {
				lc := lbkt.Cursor()
				for _, h := range []struct {
					key     []byte
					handler func([]byte, []byte)
				}{
					{
						key: labelGCContainerBackRef,
						handler: func(k, v []byte) {
							if ks := string(k); ks != string(labelGCContainerBackRef) {
								// Allow reference naming separated by . or /, ignore names
								if ks[len(labelGCContainerBackRef)] != '.' && ks[len(labelGCContainerBackRef)] != '/' {
									return
								}
							}

							bref(gcnode(metadata.ResourceContainer, ns, string(v)), n)
						},
					},
					{
						key: labelGCContentBackRef,
						handler: func(k, v []byte) {
							if ks := string(k); ks != string(labelGCContentBackRef) {
								// Allow reference naming separated by . or /, ignore names
								if ks[len(labelGCContentBackRef)] != '.' && ks[len(labelGCContentBackRef)] != '/' {
									return
								}
							}

							bref(gcnode(metadata.ResourceContent, ns, string(v)), n)
						},
					},
					{
						key: labelGCImageBackRef,
						handler: func(k, v []byte) {
							if ks := string(k); ks != string(labelGCImageBackRef) {
								// Allow reference naming separated by . or /, ignore names
								if ks[len(labelGCImageBackRef)] != '.' && ks[len(labelGCImageBackRef)] != '/' {
									return
								}
							}

							bref(gcnode(metadata.ResourceImage, ns, string(v)), n)
						},
					},
					{
						key: labelGCSnapBackRef,
						handler: func(k, v []byte) {
							snapshotter := k[len(labelGCSnapBackRef):]
							if i := bytes.IndexByte(snapshotter, '/'); i >= 0 {
								snapshotter = snapshotter[:i]
							}
							bref(gcnode(metadata.ResourceSnapshot, ns, fmt.Sprintf("%s/%s", snapshotter, v)), n)
						},
					},
					// TODO: Consider support for root/expire labels
				} {
					for k, v := lc.Seek(h.key); k != nil && bytes.HasPrefix(k, h.key); k, v = lc.Next() {
						h.handler(k, v)
					}
				}
			}
		}
	}
}

func (cc *collectionContext) Active(ns string, fn func(gc.Node)) {
	cc.ActiveWithBackRefs(ns, fn, func(gc.Node, gc.Node) {})
}

func (cc *collectionContext) Leased(ns, lease string, fn func(gc.Node)) {
	bkt := getBucket(cc.tx, []byte("v1"), []byte(ns), []byte("leases"), []byte(lease))
	if bkt != nil {
		c := bkt.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			fn(gc.Node{
				Type:      metadata.ResourceMount,
				Namespace: ns,
				Key:       string(k),
			})
		}
	}
}

func (cc *collectionContext) Remove(n gc.Node) {
	log.G(cc.ctx).WithFields(log.Fields{"namespace": n.Namespace, "name": n.Key}).Debugf("remove mount")
	if n.Type != metadata.ResourceMount {
		return
	}
	nmap, ok := cc.removed[n.Namespace]
	if ok {
		if _, ok = nmap[n.Key]; !ok {
			nmap[n.Key] = struct{}{}
		}
	} else {
		cc.removed[n.Namespace] = map[string]struct{}{
			n.Key: {},
		}
	}
}

func (cc *collectionContext) Cancel() (err error) {
	err = cc.tx.Rollback()
	cc.manager.rwlock.Unlock()
	return
}

func (cc *collectionContext) Finish() error {
	// TODO: Get list of all remaining
	remaining, err := cc.applyRemove()
	if err != nil {
		if rerr := cc.tx.Rollback(); rerr != nil {
			err = errors.Join(err, rerr)
		}
	} else {
		err = cc.tx.Commit()
	}
	if err != nil {
		cc.manager.rwlock.Unlock()
		return err
	}

	// TODO: Consider using unmount q
	cleanup, err := cc.getCleanupDirectories(remaining)

	cc.manager.rwlock.Unlock()

	if err != nil {
		return err
	}

	return cleanupAll(cc.ctx, cleanup, cc.manager.handlers)
}

func (cc *collectionContext) applyRemove() (map[uint64]struct{}, error) {
	remaining := map[uint64]struct{}{}
	v1bkt := cc.tx.Bucket([]byte("v1"))
	if v1bkt == nil {
		return remaining, nil
	}
	nsc := v1bkt.Cursor()
	for nsk, nsv := nsc.First(); nsk != nil; nsk, nsv = nsc.Next() {
		if nsv != nil {
			continue
		}
		removed := cc.removed[string(nsk)]
		nsbkt := v1bkt.Bucket(nsk)
		msbkt := nsbkt.Bucket(bucketKeyMounts)
		if msbkt == nil {
			continue
		}
		lsbkt := nsbkt.Bucket(bucketKeyLeases)
		msc := msbkt.Cursor()
		for msk, msv := msc.First(); msk != nil; msk, msv = msc.Next() {
			if msv != nil {
				continue
			}
			mbkt := msbkt.Bucket(msk)
			var remove bool
			if removed != nil {
				_, remove = removed[string(msk)]
			}

			if remove {
				if lsbkt != nil {
					lid := mbkt.Get(bucketKeyLease)
					if len(lid) > 0 {
						lbkt := lsbkt.Bucket(lid)
						if lbkt != nil {
							lbkt.Delete(msk)
							if k, _ := lbkt.Cursor().First(); k == nil {
								lsbkt.DeleteBucket(lid)
							}
						}
					}
				}
				msbkt.DeleteBucket(msk)
			} else {
				remaining[readID(mbkt)] = struct{}{}
			}
		}
	}

	return remaining, nil
}

func (cc *collectionContext) getCleanupDirectories(remaining map[uint64]struct{}) ([]string, error) {
	fd, err := cc.manager.targets.Open(".")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	dirs, err := fd.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	cleanup := []string{}
	for _, d := range dirs {
		id, err := strconv.ParseUint(d, 10, 64)
		if err != nil {
			continue
		}
		if _, ok := remaining[id]; ok {
			continue
		}
		cleanup = append(cleanup, filepath.Join(cc.manager.targets.Name(), d))
	}

	return cleanup, nil
}

func cleanupAll(ctx context.Context, roots []string, handlers map[string]mount.Handler) error {
	var errs []error
	for _, root := range roots {
		if err := unmountAll(ctx, root, handlers); err != nil {
			errs = append(errs, fmt.Errorf("unmount all failed during cleanup up %s: %w", root, err))
		} else {
			log.G(ctx).WithField("root", root).Debugf("unmounted")
		}
	}
	return errors.Join(errs...)
}

func unmountAll(ctx context.Context, root string, handlers map[string]mount.Handler) error {
	fd, err := os.Open(root)
	if err != nil {
		return err
	}

	dirs, err := fd.Readdirnames(0)
	fd.Close()
	if err != nil {
		return err
	}

	var mountErrs []error
	for i := len(dirs) - 1; i >= 0; {
		var (
			d  = dirs[i]
			mp string
			h  mount.Handler
		)
		i--

		if strings.HasSuffix(d, "-type") {
			name := d[:len(d)-5]
			if i >= 0 && dirs[i] == name {
				i--
			}
			if b, rerr := os.ReadFile(filepath.Join(root, d)); rerr == nil {
				h = handlers[string(b)]
			} else {
				return rerr
			}
			mp = filepath.Join(root, name)
		} else {
			mp = filepath.Join(root, d)
			// If type file exists, continue and try again with "-type" file
			if _, serr := os.Stat(mp + "-type"); serr == nil {
				continue
			} else if !os.IsNotExist(serr) {
				return serr
			} else {
				log.G(ctx).WithField("mount", d).Infof("missing type file, attempting unmount with no handler")
			}
		}

		if h != nil {
			err = h.Unmount(ctx, mp)
		} else {
			err = mount.Unmount(mp, 0)
		}
		if err != nil {
			// TODO: Ignore already unmounted
			mountErrs = append(mountErrs, fmt.Errorf("failure unmounting %s: %w", d, err))
		}
	}
	if len(mountErrs) > 0 {
		return errors.Join(mountErrs...)
	}

	return os.RemoveAll(root)
}
