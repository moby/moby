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

package metadata

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata/boltutil"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	bolt "go.etcd.io/bbolt"
)

const (
	inheritedLabelsPrefix = "containerd.io/snapshot/"
	labelSnapshotRef      = "containerd.io/snapshot.ref"
)

type snapshotter struct {
	snapshots.Snapshotter
	name string
	db   *DB
	l    sync.RWMutex
}

// newSnapshotter returns a new Snapshotter which namespaces the given snapshot
// using the provided name and database.
func newSnapshotter(db *DB, name string, sn snapshots.Snapshotter) *snapshotter {
	return &snapshotter{
		Snapshotter: sn,
		name:        name,
		db:          db,
	}
}

func createKey(id uint64, namespace, key string) string {
	return fmt.Sprintf("%s/%d/%s", namespace, id, key)
}

func getKey(tx *bolt.Tx, ns, name, key string) string {
	bkt := getSnapshotterBucket(tx, ns, name)
	if bkt == nil {
		return ""
	}
	bkt = bkt.Bucket([]byte(key))
	if bkt == nil {
		return ""
	}
	v := bkt.Get(bucketKeyName)
	if len(v) == 0 {
		return ""
	}
	return string(v)
}

func (s *snapshotter) resolveKey(ctx context.Context, key string) (string, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return "", err
	}

	var id string
	if err := view(ctx, s.db, func(tx *bolt.Tx) error {
		id = getKey(tx, ns, s.name, key)
		if id == "" {
			return fmt.Errorf("snapshot %v does not exist: %w", key, errdefs.ErrNotFound)
		}
		return nil
	}); err != nil {
		return "", err
	}

	return id, nil
}

func (s *snapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return snapshots.Info{}, err
	}

	var (
		bkey  string
		local = snapshots.Info{
			Name: key,
		}
	)
	if err := view(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getSnapshotterBucket(tx, ns, s.name)
		if bkt == nil {
			return fmt.Errorf("snapshot %v does not exist: %w", key, errdefs.ErrNotFound)
		}
		sbkt := bkt.Bucket([]byte(key))
		if sbkt == nil {
			return fmt.Errorf("snapshot %v does not exist: %w", key, errdefs.ErrNotFound)
		}
		local.Labels, err = boltutil.ReadLabels(sbkt)
		if err != nil {
			return fmt.Errorf("failed to read labels: %w", err)
		}
		if err := boltutil.ReadTimestamps(sbkt, &local.Created, &local.Updated); err != nil {
			return fmt.Errorf("failed to read timestamps: %w", err)
		}
		bkey = string(sbkt.Get(bucketKeyName))
		local.Parent = string(sbkt.Get(bucketKeyParent))

		return nil
	}); err != nil {
		return snapshots.Info{}, err
	}

	info, err := s.Snapshotter.Stat(ctx, bkey)
	if err != nil {
		return snapshots.Info{}, err
	}

	return overlayInfo(info, local), nil
}

func (s *snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	s.l.RLock()
	defer s.l.RUnlock()

	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return snapshots.Info{}, err
	}

	if info.Name == "" {
		return snapshots.Info{}, errdefs.ErrInvalidArgument
	}

	var (
		bkey  string
		local = snapshots.Info{
			Name: info.Name,
		}
		updated bool
	)
	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getSnapshotterBucket(tx, ns, s.name)
		if bkt == nil {
			return fmt.Errorf("snapshot %v does not exist: %w", info.Name, errdefs.ErrNotFound)
		}
		sbkt := bkt.Bucket([]byte(info.Name))
		if sbkt == nil {
			return fmt.Errorf("snapshot %v does not exist: %w", info.Name, errdefs.ErrNotFound)
		}

		local.Labels, err = boltutil.ReadLabels(sbkt)
		if err != nil {
			return fmt.Errorf("failed to read labels: %w", err)
		}
		if err := boltutil.ReadTimestamps(sbkt, &local.Created, &local.Updated); err != nil {
			return fmt.Errorf("failed to read timestamps: %w", err)
		}

		// Handle field updates
		if len(fieldpaths) > 0 {
			for _, path := range fieldpaths {
				if strings.HasPrefix(path, "labels.") {
					if local.Labels == nil {
						local.Labels = map[string]string{}
					}

					key := strings.TrimPrefix(path, "labels.")
					local.Labels[key] = info.Labels[key]
					continue
				}

				switch path {
				case "labels":
					local.Labels = info.Labels
				default:
					return fmt.Errorf("cannot update %q field on snapshot %q: %w", path, info.Name, errdefs.ErrInvalidArgument)
				}
			}
		} else {
			local.Labels = info.Labels
		}
		if err := validateSnapshot(&local); err != nil {
			return err
		}
		local.Updated = time.Now().UTC()

		if err := boltutil.WriteTimestamps(sbkt, local.Created, local.Updated); err != nil {
			return fmt.Errorf("failed to read timestamps: %w", err)
		}
		if err := boltutil.WriteLabels(sbkt, local.Labels); err != nil {
			return fmt.Errorf("failed to read labels: %w", err)
		}
		bkey = string(sbkt.Get(bucketKeyName))
		local.Parent = string(sbkt.Get(bucketKeyParent))

		inner := snapshots.Info{
			Name:   bkey,
			Labels: snapshots.FilterInheritedLabels(local.Labels),
		}

		// NOTE: Perform this inside the transaction to reduce the
		// chances of out of sync data. The backend snapshotters
		// should perform the Update as fast as possible.
		if info, err = s.Snapshotter.Update(ctx, inner, fieldpaths...); err != nil {
			return err
		}
		updated = true

		return nil
	}); err != nil {
		if updated {
			log.G(ctx).WithField("snapshotter", s.name).WithField("key", local.Name).WithError(err).Error("transaction failed after updating snapshot backend")
		}
		return snapshots.Info{}, err
	}

	return overlayInfo(info, local), nil
}

func overlayInfo(info, overlay snapshots.Info) snapshots.Info {
	// Merge info
	info.Name = overlay.Name
	info.Created = overlay.Created
	info.Updated = overlay.Updated
	info.Parent = overlay.Parent
	if info.Labels == nil {
		info.Labels = overlay.Labels
	} else {
		for k, v := range overlay.Labels {
			info.Labels[k] = v
		}
	}
	return info
}

func (s *snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	bkey, err := s.resolveKey(ctx, key)
	if err != nil {
		return snapshots.Usage{}, err
	}
	return s.Snapshotter.Usage(ctx, bkey)
}

func (s *snapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	bkey, err := s.resolveKey(ctx, key)
	if err != nil {
		return nil, err
	}
	return s.Snapshotter.Mounts(ctx, bkey)
}

func (s *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	mounts, err := s.createSnapshot(ctx, key, parent, false, opts)
	if err != nil {
		return nil, err
	}

	if s.db.dbopts.publisher != nil {
		if err := s.db.dbopts.publisher.Publish(ctx, "/snapshot/prepare", &eventstypes.SnapshotPrepare{
			Key:         key,
			Parent:      parent,
			Snapshotter: s.name,
		}); err != nil {
			return nil, err
		}
	}

	return mounts, nil
}

func (s *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, key, parent, true, opts)
}

func (s *snapshotter) createSnapshot(ctx context.Context, key, parent string, readonly bool, opts []snapshots.Opt) ([]mount.Mount, error) {
	s.l.RLock()
	defer s.l.RUnlock()

	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	var base snapshots.Info
	for _, opt := range opts {
		if err := opt(&base); err != nil {
			return nil, err
		}
	}

	if err := validateSnapshot(&base); err != nil {
		return nil, err
	}

	var (
		target  = base.Labels[labelSnapshotRef]
		bparent string
		bkey    string
		bopts   = []snapshots.Opt{
			snapshots.WithLabels(snapshots.FilterInheritedLabels(base.Labels)),
		}
	)

	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt, err := createSnapshotterBucket(tx, ns, s.name)
		if err != nil {
			return err
		}

		// Check if target exists, if so, return already exists
		if target != "" {
			if tbkt := bkt.Bucket([]byte(target)); tbkt != nil {
				return fmt.Errorf("target snapshot %q: %w", target, errdefs.ErrAlreadyExists)
			}
		}

		if bbkt := bkt.Bucket([]byte(key)); bbkt != nil {
			return fmt.Errorf("snapshot %q: %w", key, errdefs.ErrAlreadyExists)
		}

		if parent != "" {
			pbkt := bkt.Bucket([]byte(parent))
			if pbkt == nil {
				return fmt.Errorf("parent snapshot %v does not exist: %w", parent, errdefs.ErrNotFound)
			}
			bparent = string(pbkt.Get(bucketKeyName))
		}

		sid, err := bkt.NextSequence()
		if err != nil {
			return err
		}
		bkey = createKey(sid, ns, key)

		return err
	}); err != nil {
		return nil, err
	}

	var (
		m       []mount.Mount
		created string
		rerr    error
	)
	if readonly {
		m, err = s.Snapshotter.View(ctx, bkey, bparent, bopts...)
	} else {
		m, err = s.Snapshotter.Prepare(ctx, bkey, bparent, bopts...)
	}

	// An already exists error should indicate the backend found a snapshot
	// matching a provided target reference.
	if errdefs.IsAlreadyExists(err) {
		if target != "" {
			var tinfo *snapshots.Info
			filter := fmt.Sprintf(`labels."containerd.io/snapshot.ref"==%s,parent==%q`, target, bparent)
			if err := s.Snapshotter.Walk(ctx, func(ctx context.Context, i snapshots.Info) error {
				if tinfo == nil && i.Kind == snapshots.KindCommitted {
					if i.Labels["containerd.io/snapshot.ref"] != target {
						// Walk did not respect filter
						return nil
					}
					if i.Parent != bparent {
						// Walk did not respect filter
						return nil
					}
					tinfo = &i
				}
				return nil

			}, filter); err != nil {
				return nil, fmt.Errorf("failed walking backend snapshots: %w", err)
			}

			if tinfo == nil {
				return nil, fmt.Errorf("target snapshot %q in backend: %w", target, errdefs.ErrNotFound)
			}

			key = target
			bkey = tinfo.Name
			bparent = tinfo.Parent
			base.Created = tinfo.Created
			base.Updated = tinfo.Updated
			if base.Labels == nil {
				base.Labels = tinfo.Labels
			} else {
				for k, v := range tinfo.Labels {
					if _, ok := base.Labels[k]; !ok {
						base.Labels[k] = v
					}
				}
			}

			// Propagate this error after the final update
			rerr = fmt.Errorf("target snapshot %q from snapshotter: %w", target, errdefs.ErrAlreadyExists)
		} else {
			// This condition is unexpected as the key provided is expected
			// to be new and unique, return as unknown response from backend
			// to avoid confusing callers handling already exists.
			return nil, fmt.Errorf("unexpected error from snapshotter: %v: %w", err, errdefs.ErrUnknown)
		}
	} else if err != nil {
		return nil, err
	} else {
		ts := time.Now().UTC()
		base.Created = ts
		base.Updated = ts
		created = bkey
	}

	if txerr := update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getSnapshotterBucket(tx, ns, s.name)
		if bkt == nil {
			return fmt.Errorf("can not find snapshotter %q: %w", s.name, errdefs.ErrNotFound)
		}

		if err := addSnapshotLease(ctx, tx, s.name, key); err != nil {
			return err
		}

		bbkt, err := bkt.CreateBucket([]byte(key))
		if err != nil {
			if err != bolt.ErrBucketExists {
				return err
			}
			if rerr == nil {
				rerr = fmt.Errorf("snapshot %q: %w", key, errdefs.ErrAlreadyExists)
			}
			return nil
		}

		if parent != "" {
			pbkt := bkt.Bucket([]byte(parent))
			if pbkt == nil {
				return fmt.Errorf("parent snapshot %v does not exist: %w", parent, errdefs.ErrNotFound)
			}

			// Ensure the backend's parent matches the metadata store's parent
			// If it is mismatched, then a target was provided for a snapshotter
			// which has a different parent then requested.
			// NOTE: The backend snapshotter is responsible for enforcing the
			// uniqueness of the reference relationships, the metadata store
			// can only error out to prevent inconsistent data.
			if bparent != string(pbkt.Get(bucketKeyName)) {
				return fmt.Errorf("mismatched parent %s from target %s: %w", parent, target, errdefs.ErrInvalidArgument)
			}

			cbkt, err := pbkt.CreateBucketIfNotExists(bucketKeyChildren)
			if err != nil {
				return err
			}
			if err := cbkt.Put([]byte(key), nil); err != nil {
				return err
			}

			if err := bbkt.Put(bucketKeyParent, []byte(parent)); err != nil {
				return err
			}
		}

		if err := boltutil.WriteTimestamps(bbkt, base.Created, base.Updated); err != nil {
			return err
		}
		if err := boltutil.WriteLabels(bbkt, base.Labels); err != nil {
			return err
		}

		return bbkt.Put(bucketKeyName, []byte(bkey))
	}); txerr != nil {
		rerr = txerr
	}

	if rerr != nil {
		// If the created reference is not stored, attempt clean up
		if created != "" {
			if err := s.Snapshotter.Remove(ctx, created); err != nil {
				log.G(ctx).WithField("snapshotter", s.name).WithField("key", created).WithError(err).Error("failed to cleanup unreferenced snapshot")
			}
		}
		return nil, rerr
	}

	return m, nil
}

func (s *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	s.l.RLock()
	defer s.l.RUnlock()

	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	var base snapshots.Info
	for _, opt := range opts {
		if err := opt(&base); err != nil {
			return err
		}
	}

	if err := validateSnapshot(&base); err != nil {
		return err
	}

	var bname string
	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getSnapshotterBucket(tx, ns, s.name)
		if bkt == nil {
			return fmt.Errorf("can not find snapshotter %q: %w",
				s.name, errdefs.ErrNotFound)
		}

		bbkt, err := bkt.CreateBucket([]byte(name))
		if err != nil {
			if err == bolt.ErrBucketExists {
				err = fmt.Errorf("snapshot %q: %w", name, errdefs.ErrAlreadyExists)
			}
			return err
		}
		if err := addSnapshotLease(ctx, tx, s.name, name); err != nil {
			return err
		}

		obkt := bkt.Bucket([]byte(key))
		if obkt == nil {
			return fmt.Errorf("snapshot %v does not exist: %w", key, errdefs.ErrNotFound)
		}

		bkey := string(obkt.Get(bucketKeyName))

		sid, err := bkt.NextSequence()
		if err != nil {
			return err
		}

		nameKey := createKey(sid, ns, name)

		if err := bbkt.Put(bucketKeyName, []byte(nameKey)); err != nil {
			return err
		}

		parent := obkt.Get(bucketKeyParent)
		if len(parent) > 0 {
			pbkt := bkt.Bucket(parent)
			if pbkt == nil {
				return fmt.Errorf("parent snapshot %v does not exist: %w", string(parent), errdefs.ErrNotFound)
			}

			cbkt, err := pbkt.CreateBucketIfNotExists(bucketKeyChildren)
			if err != nil {
				return err
			}
			if err := cbkt.Delete([]byte(key)); err != nil {
				return err
			}
			if err := cbkt.Put([]byte(name), nil); err != nil {
				return err
			}

			if err := bbkt.Put(bucketKeyParent, parent); err != nil {
				return err
			}
		}
		ts := time.Now().UTC()
		if err := boltutil.WriteTimestamps(bbkt, ts, ts); err != nil {
			return err
		}
		if err := boltutil.WriteLabels(bbkt, base.Labels); err != nil {
			return err
		}

		if err := bkt.DeleteBucket([]byte(key)); err != nil {
			return err
		}
		if err := removeSnapshotLease(ctx, tx, s.name, key); err != nil {
			return err
		}

		inheritedOpt := snapshots.WithLabels(snapshots.FilterInheritedLabels(base.Labels))

		// NOTE: Backend snapshotters should commit fast and reliably to
		// prevent metadata store locking and minimizing rollbacks.
		// This operation should be done in the transaction to minimize the
		// risk of the committed keys becoming out of sync. If this operation
		// succeed and the overall transaction fails then the risk of out of
		// sync data is higher and may require manual cleanup.
		if err := s.Snapshotter.Commit(ctx, nameKey, bkey, inheritedOpt); err != nil {
			if errdefs.IsNotFound(err) {
				log.G(ctx).WithField("snapshotter", s.name).WithField("key", key).WithError(err).Error("uncommittable snapshot: missing in backend, snapshot should be removed")
			}
			// NOTE: Consider handling already exists here from the backend. Currently
			// already exists from the backend may be confusing to the client since it
			// may require the client to re-attempt from prepare. However, if handling
			// here it is not clear what happened with the existing backend key and
			// whether the already prepared snapshot would still be used or must be
			// discarded. It is best that all implementations of the snapshotter
			// interface behave the same, in which case the backend should handle the
			// mapping of duplicates and not error.
			return err
		}
		bname = nameKey

		return nil
	}); err != nil {
		if bname != "" {
			log.G(ctx).WithField("snapshotter", s.name).WithField("key", key).WithField("bname", bname).WithError(err).Error("uncommittable snapshot: transaction failed after commit, snapshot should be removed")

		}
		return err
	}

	if s.db.dbopts.publisher != nil {
		if err := s.db.dbopts.publisher.Publish(ctx, "/snapshot/commit", &eventstypes.SnapshotCommit{
			Key:         key,
			Name:        name,
			Snapshotter: s.name,
		}); err != nil {
			return err
		}
	}

	return nil

}

func (s *snapshotter) Remove(ctx context.Context, key string) error {
	s.l.RLock()
	defer s.l.RUnlock()

	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		var sbkt *bolt.Bucket
		bkt := getSnapshotterBucket(tx, ns, s.name)
		if bkt != nil {
			sbkt = bkt.Bucket([]byte(key))
		}
		if sbkt == nil {
			return fmt.Errorf("snapshot %v does not exist: %w", key, errdefs.ErrNotFound)
		}

		cbkt := sbkt.Bucket(bucketKeyChildren)
		if cbkt != nil {
			if child, _ := cbkt.Cursor().First(); child != nil {
				return fmt.Errorf("cannot remove snapshot with child: %w", errdefs.ErrFailedPrecondition)
			}
		}

		parent := sbkt.Get(bucketKeyParent)
		if len(parent) > 0 {
			pbkt := bkt.Bucket(parent)
			if pbkt == nil {
				return fmt.Errorf("parent snapshot %v does not exist: %w", string(parent), errdefs.ErrNotFound)
			}
			cbkt := pbkt.Bucket(bucketKeyChildren)
			if cbkt != nil {
				if err := cbkt.Delete([]byte(key)); err != nil {
					return fmt.Errorf("failed to remove child link: %w", err)
				}
			}
		}

		if err := bkt.DeleteBucket([]byte(key)); err != nil {
			return err
		}
		if err := removeSnapshotLease(ctx, tx, s.name, key); err != nil {
			return err
		}

		// Mark snapshotter as dirty for triggering garbage collection
		atomic.AddUint32(&s.db.dirty, 1)
		s.db.dirtySS[s.name] = struct{}{}

		return nil
	}); err != nil {
		return err
	}

	if s.db.dbopts.publisher != nil {
		return s.db.dbopts.publisher.Publish(ctx, "/snapshot/remove", &eventstypes.SnapshotRemove{
			Key:         key,
			Snapshotter: s.name,
		})
	}
	return nil
}

type infoPair struct {
	bkey string
	info snapshots.Info
}

func (s *snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	var (
		batchSize = 100
		pairs     = []infoPair{}
		lastKey   string
	)

	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return err
	}

	for {
		if err := view(ctx, s.db, func(tx *bolt.Tx) error {
			bkt := getSnapshotterBucket(tx, ns, s.name)
			if bkt == nil {
				return nil
			}

			c := bkt.Cursor()

			var k, v []byte
			if lastKey == "" {
				k, v = c.First()
			} else {
				k, v = c.Seek([]byte(lastKey))
			}

			for k != nil {
				if v == nil {
					if len(pairs) >= batchSize {
						break
					}
					sbkt := bkt.Bucket(k)

					pair := infoPair{
						bkey: string(sbkt.Get(bucketKeyName)),
						info: snapshots.Info{
							Name:   string(k),
							Parent: string(sbkt.Get(bucketKeyParent)),
						},
					}

					err := boltutil.ReadTimestamps(sbkt, &pair.info.Created, &pair.info.Updated)
					if err != nil {
						return err
					}
					pair.info.Labels, err = boltutil.ReadLabels(sbkt)
					if err != nil {
						return err
					}

					pairs = append(pairs, pair)
				}

				k, v = c.Next()
			}

			lastKey = string(k)

			return nil
		}); err != nil {
			return err
		}

		for _, pair := range pairs {
			info, err := s.Snapshotter.Stat(ctx, pair.bkey)
			if err != nil {
				if errdefs.IsNotFound(err) {
					continue
				}
				return err
			}

			info = overlayInfo(info, pair.info)
			if filter.Match(adaptSnapshot(info)) {
				if err := fn(ctx, info); err != nil {
					return err
				}
			}
		}

		if lastKey == "" {
			break
		}

		pairs = pairs[:0]

	}

	return nil
}

func validateSnapshot(info *snapshots.Info) error {
	for k, v := range info.Labels {
		if err := labels.Validate(k, v); err != nil {
			return fmt.Errorf("info.Labels: %w", err)
		}
	}

	return nil
}

// garbageCollect removes all snapshots that are no longer used.
func (s *snapshotter) garbageCollect(ctx context.Context) (d time.Duration, err error) {
	s.l.Lock()
	t1 := time.Now()
	defer func() {
		s.l.Unlock()
		if err == nil {
			if c, ok := s.Snapshotter.(snapshots.Cleaner); ok {
				err = c.Cleanup(ctx)
				if errdefs.IsNotImplemented(err) {
					err = nil
				}
			}
		}
		if err == nil {
			d = time.Since(t1)
		}
	}()

	seen := map[string]struct{}{}
	if err := s.db.View(func(tx *bolt.Tx) error {
		v1bkt := tx.Bucket(bucketKeyVersion)
		if v1bkt == nil {
			return nil
		}

		// iterate through each namespace
		v1c := v1bkt.Cursor()

		for k, v := v1c.First(); k != nil; k, v = v1c.Next() {
			if v != nil {
				continue
			}

			sbkt := v1bkt.Bucket(k).Bucket(bucketKeyObjectSnapshots)
			if sbkt == nil {
				continue
			}

			// Load specific snapshotter
			ssbkt := sbkt.Bucket([]byte(s.name))
			if ssbkt == nil {
				continue
			}

			if err := ssbkt.ForEach(func(sk, sv []byte) error {
				if sv == nil {
					bkey := ssbkt.Bucket(sk).Get(bucketKeyName)
					if len(bkey) > 0 {
						seen[string(bkey)] = struct{}{}
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return 0, err
	}

	roots, err := s.walkTree(ctx, seen)
	if err != nil {
		return 0, err
	}

	// TODO: Unlock before removal (once nodes are fully unavailable).
	// This could be achieved through doing prune inside the lock
	// and having a cleanup method which actually performs the
	// deletions on the snapshotters which support it.

	for _, node := range roots {
		if err := s.pruneBranch(ctx, node); err != nil {
			return 0, err
		}
	}

	return
}

type treeNode struct {
	info     snapshots.Info
	remove   bool
	children []*treeNode
}

func (s *snapshotter) walkTree(ctx context.Context, seen map[string]struct{}) ([]*treeNode, error) {
	roots := []*treeNode{}
	nodes := map[string]*treeNode{}

	if err := s.Snapshotter.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		_, isSeen := seen[info.Name]
		node, ok := nodes[info.Name]
		if !ok {
			node = &treeNode{}
			nodes[info.Name] = node
		}

		node.remove = !isSeen
		node.info = info

		if info.Parent == "" {
			roots = append(roots, node)
		} else {
			parent, ok := nodes[info.Parent]
			if !ok {
				parent = &treeNode{}
				nodes[info.Parent] = parent
			}
			parent.children = append(parent.children, node)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return roots, nil
}

func (s *snapshotter) pruneBranch(ctx context.Context, node *treeNode) error {
	for _, child := range node.children {
		if err := s.pruneBranch(ctx, child); err != nil {
			return err
		}
	}

	if node.remove {
		logger := log.G(ctx).WithField("snapshotter", s.name)
		if err := s.Snapshotter.Remove(ctx, node.info.Name); err != nil {
			if !errdefs.IsFailedPrecondition(err) {
				return err
			}
			logger.WithError(err).WithField("key", node.info.Name).Warnf("failed to remove snapshot")
		} else {
			logger.WithField("key", node.info.Name).Debug("removed snapshot")
		}
	}

	return nil
}

// Close closes s.Snapshotter but not db
func (s *snapshotter) Close() error {
	return s.Snapshotter.Close()
}
