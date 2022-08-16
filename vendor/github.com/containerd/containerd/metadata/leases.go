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
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/metadata/boltutil"
	"github.com/containerd/containerd/namespaces"
	digest "github.com/opencontainers/go-digest"
	bolt "go.etcd.io/bbolt"
)

// leaseManager manages the create/delete lifecycle of leases
// and also returns existing leases
type leaseManager struct {
	db *DB
}

// NewLeaseManager creates a new lease manager for managing leases using
// the provided database transaction.
func NewLeaseManager(db *DB) leases.Manager {
	return &leaseManager{
		db: db,
	}
}

// Create creates a new lease using the provided lease
func (lm *leaseManager) Create(ctx context.Context, opts ...leases.Opt) (leases.Lease, error) {
	var l leases.Lease
	for _, opt := range opts {
		if err := opt(&l); err != nil {
			return leases.Lease{}, err
		}
	}
	if l.ID == "" {
		return leases.Lease{}, errors.New("lease id must be provided")
	}

	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return leases.Lease{}, err
	}

	if err := update(ctx, lm.db, func(tx *bolt.Tx) error {
		topbkt, err := createBucketIfNotExists(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases)
		if err != nil {
			return err
		}

		txbkt, err := topbkt.CreateBucket([]byte(l.ID))
		if err != nil {
			if err == bolt.ErrBucketExists {
				err = errdefs.ErrAlreadyExists
			}
			return fmt.Errorf("lease %q: %w", l.ID, err)
		}

		t := time.Now().UTC()
		createdAt, err := t.MarshalBinary()
		if err != nil {
			return err
		}
		if err := txbkt.Put(bucketKeyCreatedAt, createdAt); err != nil {
			return err
		}

		if l.Labels != nil {
			if err := boltutil.WriteLabels(txbkt, l.Labels); err != nil {
				return err
			}
		}
		l.CreatedAt = t

		return nil
	}); err != nil {
		return leases.Lease{}, err
	}
	return l, nil
}

// Delete deletes the lease with the provided lease ID
func (lm *leaseManager) Delete(ctx context.Context, lease leases.Lease, _ ...leases.DeleteOpt) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	return update(ctx, lm.db, func(tx *bolt.Tx) error {
		topbkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases)
		if topbkt == nil {
			return fmt.Errorf("lease %q: %w", lease.ID, errdefs.ErrNotFound)
		}
		if err := topbkt.DeleteBucket([]byte(lease.ID)); err != nil {
			if err == bolt.ErrBucketNotFound {
				err = fmt.Errorf("lease %q: %w", lease.ID, errdefs.ErrNotFound)
			}
			return err
		}

		atomic.AddUint32(&lm.db.dirty, 1)

		return nil
	})
}

// List lists all active leases
func (lm *leaseManager) List(ctx context.Context, fs ...string) ([]leases.Lease, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", err.Error(), errdefs.ErrInvalidArgument)
	}

	var ll []leases.Lease

	if err := view(ctx, lm.db, func(tx *bolt.Tx) error {
		topbkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases)
		if topbkt == nil {
			return nil
		}

		return topbkt.ForEach(func(k, v []byte) error {
			if v != nil {
				return nil
			}
			txbkt := topbkt.Bucket(k)

			l := leases.Lease{
				ID: string(k),
			}

			if v := txbkt.Get(bucketKeyCreatedAt); v != nil {
				t := &l.CreatedAt
				if err := t.UnmarshalBinary(v); err != nil {
					return err
				}
			}

			labels, err := boltutil.ReadLabels(txbkt)
			if err != nil {
				return err
			}
			l.Labels = labels

			if filter.Match(adaptLease(l)) {
				ll = append(ll, l)
			}

			return nil
		})
	}); err != nil {
		return nil, err
	}

	return ll, nil
}

// AddResource references the resource by the provided lease.
func (lm *leaseManager) AddResource(ctx context.Context, lease leases.Lease, r leases.Resource) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	return update(ctx, lm.db, func(tx *bolt.Tx) error {
		topbkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lease.ID))
		if topbkt == nil {
			return fmt.Errorf("lease %q: %w", lease.ID, errdefs.ErrNotFound)
		}

		keys, ref, err := parseLeaseResource(r)
		if err != nil {
			return err
		}

		bkt := topbkt
		for _, key := range keys {
			bkt, err = bkt.CreateBucketIfNotExists([]byte(key))
			if err != nil {
				return err
			}
		}
		return bkt.Put([]byte(ref), nil)
	})
}

// DeleteResource dereferences the resource by the provided lease.
func (lm *leaseManager) DeleteResource(ctx context.Context, lease leases.Lease, r leases.Resource) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	return update(ctx, lm.db, func(tx *bolt.Tx) error {
		topbkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lease.ID))
		if topbkt == nil {
			return fmt.Errorf("lease %q: %w", lease.ID, errdefs.ErrNotFound)
		}

		keys, ref, err := parseLeaseResource(r)
		if err != nil {
			return err
		}

		bkt := topbkt
		for _, key := range keys {
			if bkt == nil {
				break
			}
			bkt = bkt.Bucket([]byte(key))
		}

		if bkt != nil {
			if err := bkt.Delete([]byte(ref)); err != nil {
				return err
			}
		}

		atomic.AddUint32(&lm.db.dirty, 1)

		return nil
	})
}

// ListResources lists all the resources referenced by the lease.
func (lm *leaseManager) ListResources(ctx context.Context, lease leases.Lease) ([]leases.Resource, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	var rs []leases.Resource

	if err := view(ctx, lm.db, func(tx *bolt.Tx) error {

		topbkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lease.ID))
		if topbkt == nil {
			return fmt.Errorf("lease %q: %w", lease.ID, errdefs.ErrNotFound)
		}

		// content resources
		if cbkt := topbkt.Bucket(bucketKeyObjectContent); cbkt != nil {
			if err := cbkt.ForEach(func(k, _ []byte) error {
				rs = append(rs, leases.Resource{
					ID:   string(k),
					Type: string(bucketKeyObjectContent),
				})

				return nil
			}); err != nil {
				return err
			}
		}

		// ingest resources
		if lbkt := topbkt.Bucket(bucketKeyObjectIngests); lbkt != nil {
			if err := lbkt.ForEach(func(k, _ []byte) error {
				rs = append(rs, leases.Resource{
					ID:   string(k),
					Type: string(bucketKeyObjectIngests),
				})

				return nil
			}); err != nil {
				return err
			}
		}

		// snapshot resources
		if sbkt := topbkt.Bucket(bucketKeyObjectSnapshots); sbkt != nil {
			if err := sbkt.ForEach(func(sk, sv []byte) error {
				if sv != nil {
					return nil
				}

				snbkt := sbkt.Bucket(sk)
				return snbkt.ForEach(func(k, _ []byte) error {
					rs = append(rs, leases.Resource{
						ID:   string(k),
						Type: fmt.Sprintf("%s/%s", bucketKeyObjectSnapshots, sk),
					})
					return nil
				})
			}); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return rs, nil
}

func addSnapshotLease(ctx context.Context, tx *bolt.Tx, snapshotter, key string) error {
	lid, ok := leases.FromContext(ctx)
	if !ok {
		return nil
	}

	namespace, ok := namespaces.Namespace(ctx)
	if !ok {
		panic("namespace must already be checked")
	}

	bkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lid))
	if bkt == nil {
		return fmt.Errorf("lease does not exist: %w", errdefs.ErrNotFound)
	}

	bkt, err := bkt.CreateBucketIfNotExists(bucketKeyObjectSnapshots)
	if err != nil {
		return err
	}

	bkt, err = bkt.CreateBucketIfNotExists([]byte(snapshotter))
	if err != nil {
		return err
	}

	return bkt.Put([]byte(key), nil)
}

func removeSnapshotLease(ctx context.Context, tx *bolt.Tx, snapshotter, key string) error {
	lid, ok := leases.FromContext(ctx)
	if !ok {
		return nil
	}

	namespace, ok := namespaces.Namespace(ctx)
	if !ok {
		panic("namespace must already be checked")
	}

	bkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lid), bucketKeyObjectSnapshots, []byte(snapshotter))
	if bkt == nil {
		// Key does not exist so we return nil
		return nil
	}

	return bkt.Delete([]byte(key))
}

func addContentLease(ctx context.Context, tx *bolt.Tx, dgst digest.Digest) error {
	lid, ok := leases.FromContext(ctx)
	if !ok {
		return nil
	}

	namespace, ok := namespaces.Namespace(ctx)
	if !ok {
		panic("namespace must already be required")
	}

	bkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lid))
	if bkt == nil {
		return fmt.Errorf("lease does not exist: %w", errdefs.ErrNotFound)
	}

	bkt, err := bkt.CreateBucketIfNotExists(bucketKeyObjectContent)
	if err != nil {
		return err
	}

	return bkt.Put([]byte(dgst.String()), nil)
}

func removeContentLease(ctx context.Context, tx *bolt.Tx, dgst digest.Digest) error {
	lid, ok := leases.FromContext(ctx)
	if !ok {
		return nil
	}

	namespace, ok := namespaces.Namespace(ctx)
	if !ok {
		panic("namespace must already be checked")
	}

	bkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lid), bucketKeyObjectContent)
	if bkt == nil {
		// Key does not exist so we return nil
		return nil
	}

	return bkt.Delete([]byte(dgst.String()))
}

func addIngestLease(ctx context.Context, tx *bolt.Tx, ref string) (bool, error) {
	lid, ok := leases.FromContext(ctx)
	if !ok {
		return false, nil
	}

	namespace, ok := namespaces.Namespace(ctx)
	if !ok {
		panic("namespace must already be required")
	}

	bkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lid))
	if bkt == nil {
		return false, fmt.Errorf("lease does not exist: %w", errdefs.ErrNotFound)
	}

	bkt, err := bkt.CreateBucketIfNotExists(bucketKeyObjectIngests)
	if err != nil {
		return false, err
	}

	if err := bkt.Put([]byte(ref), nil); err != nil {
		return false, err
	}

	return true, nil
}

func removeIngestLease(ctx context.Context, tx *bolt.Tx, ref string) error {
	lid, ok := leases.FromContext(ctx)
	if !ok {
		return nil
	}

	namespace, ok := namespaces.Namespace(ctx)
	if !ok {
		panic("namespace must already be checked")
	}

	bkt := getBucket(tx, bucketKeyVersion, []byte(namespace), bucketKeyObjectLeases, []byte(lid), bucketKeyObjectIngests)
	if bkt == nil {
		// Key does not exist so we return nil
		return nil
	}

	return bkt.Delete([]byte(ref))
}

func parseLeaseResource(r leases.Resource) ([]string, string, error) {
	var (
		ref  = r.ID
		typ  = r.Type
		keys = strings.Split(typ, "/")
	)

	switch k := keys[0]; k {
	case string(bucketKeyObjectContent),
		string(bucketKeyObjectIngests):

		if len(keys) != 1 {
			return nil, "", fmt.Errorf("invalid resource type %s: %w", typ, errdefs.ErrInvalidArgument)
		}

		if k == string(bucketKeyObjectContent) {
			dgst, err := digest.Parse(ref)
			if err != nil {
				return nil, "", fmt.Errorf("invalid content resource id %s: %v: %w", ref, err, errdefs.ErrInvalidArgument)
			}
			ref = dgst.String()
		}
	case string(bucketKeyObjectSnapshots):
		if len(keys) != 2 {
			return nil, "", fmt.Errorf("invalid snapshot resource type %s: %w", typ, errdefs.ErrInvalidArgument)
		}
	default:
		return nil, "", fmt.Errorf("resource type %s not supported yet: %w", typ, errdefs.ErrNotImplemented)
	}

	return keys, ref, nil
}
