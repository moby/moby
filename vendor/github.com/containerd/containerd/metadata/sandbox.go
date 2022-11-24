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
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/identifiers"
	"github.com/containerd/containerd/metadata/boltutil"
	"github.com/containerd/containerd/namespaces"
	api "github.com/containerd/containerd/sandbox"
	"github.com/containerd/typeurl/v2"
	"go.etcd.io/bbolt"
)

type sandboxStore struct {
	db *DB
}

var _ api.Store = (*sandboxStore)(nil)

// NewSandboxStore creates a datababase client for sandboxes
func NewSandboxStore(db *DB) api.Store {
	return &sandboxStore{db: db}
}

// Create a sandbox record in the store
func (s *sandboxStore) Create(ctx context.Context, sandbox api.Sandbox) (api.Sandbox, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return api.Sandbox{}, err
	}

	sandbox.CreatedAt = time.Now().UTC()
	sandbox.UpdatedAt = sandbox.CreatedAt

	if err := s.validate(&sandbox); err != nil {
		return api.Sandbox{}, fmt.Errorf("failed to validate sandbox: %w", err)
	}

	if err := s.db.Update(func(tx *bbolt.Tx) error {
		parent, err := createSandboxBucket(tx, ns)
		if err != nil {
			return fmt.Errorf("create error: %w", err)
		}

		if err := s.write(parent, &sandbox, false); err != nil {
			return fmt.Errorf("write error: %w", err)
		}

		return nil
	}); err != nil {
		return api.Sandbox{}, err
	}

	return sandbox, nil
}

// Update the sandbox with the provided sandbox object and fields
func (s *sandboxStore) Update(ctx context.Context, sandbox api.Sandbox, fieldpaths ...string) (api.Sandbox, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return api.Sandbox{}, err
	}

	ret := api.Sandbox{}
	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		parent := getSandboxBucket(tx, ns)
		if parent == nil {
			return fmt.Errorf("no sandbox buckets: %w", errdefs.ErrNotFound)
		}

		updated, err := s.read(parent, []byte(sandbox.ID))
		if err != nil {
			return err
		}

		if len(fieldpaths) == 0 {
			fieldpaths = []string{"labels", "extensions", "spec", "runtime"}

			if updated.Runtime.Name != sandbox.Runtime.Name {
				return fmt.Errorf("sandbox.Runtime.Name field is immutable: %w", errdefs.ErrInvalidArgument)
			}
		}

		for _, path := range fieldpaths {
			if strings.HasPrefix(path, "labels.") {
				if updated.Labels == nil {
					updated.Labels = map[string]string{}
				}

				key := strings.TrimPrefix(path, "labels.")
				updated.Labels[key] = sandbox.Labels[key]
				continue
			} else if strings.HasPrefix(path, "extensions.") {
				if updated.Extensions == nil {
					updated.Extensions = map[string]typeurl.Any{}
				}

				key := strings.TrimPrefix(path, "extensions.")
				updated.Extensions[key] = sandbox.Extensions[key]
				continue
			}

			switch path {
			case "labels":
				updated.Labels = sandbox.Labels
			case "extensions":
				updated.Extensions = sandbox.Extensions
			case "runtime":
				updated.Runtime = sandbox.Runtime
			case "spec":
				updated.Spec = sandbox.Spec
			default:
				return fmt.Errorf("cannot update %q field on sandbox %q: %w", path, sandbox.ID, errdefs.ErrInvalidArgument)
			}
		}

		updated.UpdatedAt = time.Now().UTC()

		if err := s.validate(&updated); err != nil {
			return err
		}

		if err := s.write(parent, &updated, true); err != nil {
			return err
		}

		ret = updated
		return nil
	}); err != nil {
		return api.Sandbox{}, err
	}

	return ret, nil
}

// Get sandbox metadata using the id
func (s *sandboxStore) Get(ctx context.Context, id string) (api.Sandbox, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return api.Sandbox{}, err
	}

	ret := api.Sandbox{}
	if err := view(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBucket(tx, ns)
		if bucket == nil {
			return fmt.Errorf("no sandbox buckets: %w", errdefs.ErrNotFound)
		}

		out, err := s.read(bucket, []byte(id))
		if err != nil {
			return err
		}

		ret = out
		return nil
	}); err != nil {
		return api.Sandbox{}, err
	}

	return ret, nil
}

// List returns sandboxes that match one or more of the provided filters
func (s *sandboxStore) List(ctx context.Context, fields ...string) ([]api.Sandbox, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	filter, err := filters.ParseAll(fields...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", err.Error(), errdefs.ErrInvalidArgument)
	}

	var (
		list []api.Sandbox
	)

	if err := view(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBucket(tx, ns)
		if bucket == nil {
			// We haven't created any sandboxes yet, just return empty list
			return nil
		}

		if err := bucket.ForEach(func(k, v []byte) error {
			info, err := s.read(bucket, k)
			if err != nil {
				return fmt.Errorf("failed to read bucket %q: %w", string(k), err)
			}

			if filter.Match(adaptSandbox(&info)) {
				list = append(list, info)
			}

			return nil
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return list, nil
}

// Delete a sandbox from metadata store using the id
func (s *sandboxStore) Delete(ctx context.Context, id string) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		buckets := getSandboxBucket(tx, ns)
		if buckets == nil {
			return fmt.Errorf("no sandbox buckets: %w", errdefs.ErrNotFound)
		}

		if err := buckets.DeleteBucket([]byte(id)); err != nil {
			return fmt.Errorf("failed to delete sandbox %q: %w", id, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *sandboxStore) write(parent *bbolt.Bucket, instance *api.Sandbox, overwrite bool) error {
	var (
		bucket *bbolt.Bucket
		err    error
		id     = []byte(instance.ID)
	)

	if overwrite {
		bucket, err = parent.CreateBucketIfNotExists(id)
		if err != nil {
			return err
		}
	} else {
		bucket = parent.Bucket(id)
		if bucket != nil {
			return fmt.Errorf("sandbox bucket %q already exists: %w", instance.ID, errdefs.ErrAlreadyExists)
		}

		bucket, err = parent.CreateBucket(id)
		if err != nil {
			return err
		}
	}

	if err := boltutil.WriteTimestamps(bucket, instance.CreatedAt, instance.UpdatedAt); err != nil {
		return err
	}

	if err := boltutil.WriteLabels(bucket, instance.Labels); err != nil {
		return err
	}

	if err := boltutil.WriteExtensions(bucket, instance.Extensions); err != nil {
		return err
	}

	if err := boltutil.WriteAny(bucket, bucketKeySpec, instance.Spec); err != nil {
		return err
	}

	runtimeBucket, err := bucket.CreateBucketIfNotExists(bucketKeyRuntime)
	if err != nil {
		return err
	}

	if err := runtimeBucket.Put(bucketKeyName, []byte(instance.Runtime.Name)); err != nil {
		return err
	}

	if err := boltutil.WriteAny(runtimeBucket, bucketKeyOptions, instance.Runtime.Options); err != nil {
		return err
	}

	return nil
}

func (s *sandboxStore) read(parent *bbolt.Bucket, id []byte) (api.Sandbox, error) {
	var (
		inst api.Sandbox
		err  error
	)

	bucket := parent.Bucket(id)
	if bucket == nil {
		return api.Sandbox{}, fmt.Errorf("bucket %q not found: %w", id, errdefs.ErrNotFound)
	}

	inst.ID = string(id)

	inst.Labels, err = boltutil.ReadLabels(bucket)
	if err != nil {
		return api.Sandbox{}, err
	}

	if err := boltutil.ReadTimestamps(bucket, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
		return api.Sandbox{}, err
	}

	inst.Spec, err = boltutil.ReadAny(bucket, bucketKeySpec)
	if err != nil {
		return api.Sandbox{}, err
	}

	runtimeBucket := bucket.Bucket(bucketKeyRuntime)
	if runtimeBucket == nil {
		return api.Sandbox{}, errors.New("no runtime bucket")
	}

	inst.Runtime.Name = string(runtimeBucket.Get(bucketKeyName))
	inst.Runtime.Options, err = boltutil.ReadAny(runtimeBucket, bucketKeyOptions)
	if err != nil {
		return api.Sandbox{}, err
	}

	inst.Extensions, err = boltutil.ReadExtensions(bucket)
	if err != nil {
		return api.Sandbox{}, err
	}

	return inst, nil
}

func (s *sandboxStore) validate(new *api.Sandbox) error {
	if err := identifiers.Validate(new.ID); err != nil {
		return fmt.Errorf("invalid sandbox ID: %w", err)
	}

	if new.CreatedAt.IsZero() {
		return fmt.Errorf("creation date must not be zero: %w", errdefs.ErrInvalidArgument)
	}

	if new.UpdatedAt.IsZero() {
		return fmt.Errorf("updated date must not be zero: %w", errdefs.ErrInvalidArgument)
	}

	return nil
}
