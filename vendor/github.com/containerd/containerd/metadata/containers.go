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
	"strings"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/identifiers"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/metadata/boltutil"
	"github.com/containerd/containerd/namespaces"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

type containerStore struct {
	db *DB
}

// NewContainerStore returns a Store backed by an underlying bolt DB
func NewContainerStore(db *DB) containers.Store {
	return &containerStore{
		db: db,
	}
}

func (s *containerStore) Get(ctx context.Context, id string) (containers.Container, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return containers.Container{}, err
	}

	container := containers.Container{ID: id}

	if err := view(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getContainerBucket(tx, namespace, id)
		if bkt == nil {
			return errors.Wrapf(errdefs.ErrNotFound, "container %q in namespace %q", id, namespace)
		}

		if err := readContainer(&container, bkt); err != nil {
			return errors.Wrapf(err, "failed to read container %q", id)
		}

		return nil
	}); err != nil {
		return containers.Container{}, err
	}

	return container, nil
}

func (s *containerStore) List(ctx context.Context, fs ...string) ([]containers.Container, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, err.Error())
	}

	var m []containers.Container

	if err := view(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getContainersBucket(tx, namespace)
		if bkt == nil {
			return nil // empty store
		}

		return bkt.ForEach(func(k, v []byte) error {
			cbkt := bkt.Bucket(k)
			if cbkt == nil {
				return nil
			}
			container := containers.Container{ID: string(k)}

			if err := readContainer(&container, cbkt); err != nil {
				return errors.Wrapf(err, "failed to read container %q", string(k))
			}

			if filter.Match(adaptContainer(container)) {
				m = append(m, container)
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}

	return m, nil
}

func (s *containerStore) Create(ctx context.Context, container containers.Container) (containers.Container, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return containers.Container{}, err
	}

	if err := validateContainer(&container); err != nil {
		return containers.Container{}, errors.Wrap(err, "create container failed validation")
	}

	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt, err := createContainersBucket(tx, namespace)
		if err != nil {
			return err
		}

		cbkt, err := bkt.CreateBucket([]byte(container.ID))
		if err != nil {
			if err == bolt.ErrBucketExists {
				err = errors.Wrapf(errdefs.ErrAlreadyExists, "container %q", container.ID)
			}
			return err
		}

		container.CreatedAt = time.Now().UTC()
		container.UpdatedAt = container.CreatedAt
		if err := writeContainer(cbkt, &container); err != nil {
			return errors.Wrapf(err, "failed to write container %q", container.ID)
		}

		return nil
	}); err != nil {
		return containers.Container{}, err
	}

	return container, nil
}

func (s *containerStore) Update(ctx context.Context, container containers.Container, fieldpaths ...string) (containers.Container, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return containers.Container{}, err
	}

	if container.ID == "" {
		return containers.Container{}, errors.Wrapf(errdefs.ErrInvalidArgument, "must specify a container id")
	}

	var updated containers.Container
	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getContainersBucket(tx, namespace)
		if bkt == nil {
			return errors.Wrapf(errdefs.ErrNotFound, "cannot update container %q in namespace %q", container.ID, namespace)
		}

		cbkt := bkt.Bucket([]byte(container.ID))
		if cbkt == nil {
			return errors.Wrapf(errdefs.ErrNotFound, "container %q", container.ID)
		}

		if err := readContainer(&updated, cbkt); err != nil {
			return errors.Wrapf(err, "failed to read container %q", container.ID)
		}
		createdat := updated.CreatedAt
		updated.ID = container.ID

		if len(fieldpaths) == 0 {
			// only allow updates to these field on full replace.
			fieldpaths = []string{"labels", "spec", "extensions", "image", "snapshotkey"}

			// Fields that are immutable must cause an error when no field paths
			// are provided. This allows these fields to become mutable in the
			// future.
			if updated.Snapshotter != container.Snapshotter {
				return errors.Wrapf(errdefs.ErrInvalidArgument, "container.Snapshotter field is immutable")
			}

			if updated.Runtime.Name != container.Runtime.Name {
				return errors.Wrapf(errdefs.ErrInvalidArgument, "container.Runtime.Name field is immutable")
			}
		}

		// apply the field mask. If you update this code, you better follow the
		// field mask rules in field_mask.proto. If you don't know what this
		// is, do not update this code.
		for _, path := range fieldpaths {
			if strings.HasPrefix(path, "labels.") {
				if updated.Labels == nil {
					updated.Labels = map[string]string{}
				}
				key := strings.TrimPrefix(path, "labels.")
				updated.Labels[key] = container.Labels[key]
				continue
			}

			if strings.HasPrefix(path, "extensions.") {
				if updated.Extensions == nil {
					updated.Extensions = map[string]types.Any{}
				}
				key := strings.TrimPrefix(path, "extensions.")
				updated.Extensions[key] = container.Extensions[key]
				continue
			}

			switch path {
			case "labels":
				updated.Labels = container.Labels
			case "spec":
				updated.Spec = container.Spec
			case "extensions":
				updated.Extensions = container.Extensions
			case "image":
				updated.Image = container.Image
			case "snapshotkey":
				updated.SnapshotKey = container.SnapshotKey
			default:
				return errors.Wrapf(errdefs.ErrInvalidArgument, "cannot update %q field on %q", path, container.ID)
			}
		}

		if err := validateContainer(&updated); err != nil {
			return errors.Wrap(err, "update failed validation")
		}

		updated.CreatedAt = createdat
		updated.UpdatedAt = time.Now().UTC()
		if err := writeContainer(cbkt, &updated); err != nil {
			return errors.Wrapf(err, "failed to write container %q", container.ID)
		}

		return nil
	}); err != nil {
		return containers.Container{}, err
	}

	return updated, nil
}

func (s *containerStore) Delete(ctx context.Context, id string) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	return update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getContainersBucket(tx, namespace)
		if bkt == nil {
			return errors.Wrapf(errdefs.ErrNotFound, "cannot delete container %q in namespace %q", id, namespace)
		}

		if err := bkt.DeleteBucket([]byte(id)); err != nil {
			if err == bolt.ErrBucketNotFound {
				err = errors.Wrapf(errdefs.ErrNotFound, "container %v", id)
			}
			return err
		}

		atomic.AddUint32(&s.db.dirty, 1)

		return nil
	})
}

func validateContainer(container *containers.Container) error {
	if err := identifiers.Validate(container.ID); err != nil {
		return errors.Wrap(err, "container.ID")
	}

	for k := range container.Extensions {
		if k == "" {
			return errors.Wrapf(errdefs.ErrInvalidArgument, "container.Extension keys must not be zero-length")
		}
	}

	// image has no validation
	for k, v := range container.Labels {
		if err := labels.Validate(k, v); err != nil {
			return errors.Wrapf(err, "containers.Labels")
		}
	}

	if container.Runtime.Name == "" {
		return errors.Wrapf(errdefs.ErrInvalidArgument, "container.Runtime.Name must be set")
	}

	if container.Spec == nil {
		return errors.Wrapf(errdefs.ErrInvalidArgument, "container.Spec must be set")
	}

	if container.SnapshotKey != "" && container.Snapshotter == "" {
		return errors.Wrapf(errdefs.ErrInvalidArgument, "container.Snapshotter must be set if container.SnapshotKey is set")
	}

	return nil
}

func readContainer(container *containers.Container, bkt *bolt.Bucket) error {
	labels, err := boltutil.ReadLabels(bkt)
	if err != nil {
		return err
	}
	container.Labels = labels

	if err := boltutil.ReadTimestamps(bkt, &container.CreatedAt, &container.UpdatedAt); err != nil {
		return err
	}

	return bkt.ForEach(func(k, v []byte) error {
		switch string(k) {
		case string(bucketKeyImage):
			container.Image = string(v)
		case string(bucketKeyRuntime):
			rbkt := bkt.Bucket(bucketKeyRuntime)
			if rbkt == nil {
				return nil // skip runtime. should be an error?
			}

			n := rbkt.Get(bucketKeyName)
			if n != nil {
				container.Runtime.Name = string(n)
			}

			any, err := boltutil.ReadAny(rbkt, bucketKeyOptions)
			if err != nil {
				return err
			}
			container.Runtime.Options = any
		case string(bucketKeySpec):
			var any types.Any
			if err := proto.Unmarshal(v, &any); err != nil {
				return err
			}
			container.Spec = &any
		case string(bucketKeySnapshotKey):
			container.SnapshotKey = string(v)
		case string(bucketKeySnapshotter):
			container.Snapshotter = string(v)
		case string(bucketKeyExtensions):
			extensions, err := boltutil.ReadExtensions(bkt)
			if err != nil {
				return err
			}

			container.Extensions = extensions
		}

		return nil
	})
}

func writeContainer(bkt *bolt.Bucket, container *containers.Container) error {
	if err := boltutil.WriteTimestamps(bkt, container.CreatedAt, container.UpdatedAt); err != nil {
		return err
	}

	if err := boltutil.WriteAny(bkt, bucketKeySpec, container.Spec); err != nil {
		return err
	}

	for _, v := range [][2][]byte{
		{bucketKeyImage, []byte(container.Image)},
		{bucketKeySnapshotter, []byte(container.Snapshotter)},
		{bucketKeySnapshotKey, []byte(container.SnapshotKey)},
	} {
		if err := bkt.Put(v[0], v[1]); err != nil {
			return err
		}
	}

	if rbkt := bkt.Bucket(bucketKeyRuntime); rbkt != nil {
		if err := bkt.DeleteBucket(bucketKeyRuntime); err != nil {
			return err
		}
	}

	rbkt, err := bkt.CreateBucket(bucketKeyRuntime)
	if err != nil {
		return err
	}

	if err := rbkt.Put(bucketKeyName, []byte(container.Runtime.Name)); err != nil {
		return err
	}

	if err := boltutil.WriteExtensions(bkt, container.Extensions); err != nil {
		return err
	}

	if err := boltutil.WriteAny(rbkt, bucketKeyOptions, container.Runtime.Options); err != nil {
		return err
	}

	return boltutil.WriteLabels(bkt, container.Labels)
}
