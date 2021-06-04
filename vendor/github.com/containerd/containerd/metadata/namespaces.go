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

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/identifiers"
	l "github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/namespaces"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

type namespaceStore struct {
	tx *bolt.Tx
}

// NewNamespaceStore returns a store backed by a bolt DB
func NewNamespaceStore(tx *bolt.Tx) namespaces.Store {
	return &namespaceStore{tx: tx}
}

func (s *namespaceStore) Create(ctx context.Context, namespace string, labels map[string]string) error {
	topbkt, err := createBucketIfNotExists(s.tx, bucketKeyVersion)
	if err != nil {
		return err
	}

	if err := identifiers.Validate(namespace); err != nil {
		return err
	}

	for k, v := range labels {
		if err := l.Validate(k, v); err != nil {
			return errors.Wrapf(err, "namespace.Labels")
		}
	}

	// provides the already exists error.
	bkt, err := topbkt.CreateBucket([]byte(namespace))
	if err != nil {
		if err == bolt.ErrBucketExists {
			return errors.Wrapf(errdefs.ErrAlreadyExists, "namespace %q", namespace)
		}

		return err
	}

	lbkt, err := bkt.CreateBucketIfNotExists(bucketKeyObjectLabels)
	if err != nil {
		return err
	}

	for k, v := range labels {
		if err := lbkt.Put([]byte(k), []byte(v)); err != nil {
			return err
		}
	}

	return nil
}

func (s *namespaceStore) Labels(ctx context.Context, namespace string) (map[string]string, error) {
	labels := map[string]string{}

	bkt := getNamespaceLabelsBucket(s.tx, namespace)
	if bkt == nil {
		return labels, nil
	}

	if err := bkt.ForEach(func(k, v []byte) error {
		labels[string(k)] = string(v)
		return nil
	}); err != nil {
		return nil, err
	}

	return labels, nil
}

func (s *namespaceStore) SetLabel(ctx context.Context, namespace, key, value string) error {
	if err := l.Validate(key, value); err != nil {
		return errors.Wrapf(err, "namespace.Labels")
	}

	return withNamespacesLabelsBucket(s.tx, namespace, func(bkt *bolt.Bucket) error {
		if value == "" {
			return bkt.Delete([]byte(key))
		}

		return bkt.Put([]byte(key), []byte(value))
	})

}

func (s *namespaceStore) List(ctx context.Context) ([]string, error) {
	bkt := getBucket(s.tx, bucketKeyVersion)
	if bkt == nil {
		return nil, nil // no namespaces!
	}

	var namespaces []string
	if err := bkt.ForEach(func(k, v []byte) error {
		if v != nil {
			return nil // not a bucket
		}

		namespaces = append(namespaces, string(k))
		return nil
	}); err != nil {
		return nil, err
	}

	return namespaces, nil
}

func (s *namespaceStore) Delete(ctx context.Context, namespace string, opts ...namespaces.DeleteOpts) error {
	i := &namespaces.DeleteInfo{
		Name: namespace,
	}
	for _, o := range opts {
		if err := o(ctx, i); err != nil {
			return err
		}
	}
	bkt := getBucket(s.tx, bucketKeyVersion)
	types, err := s.listNs(namespace)
	if err != nil {
		return err
	}

	if len(types) > 0 {
		return errors.Wrapf(
			errdefs.ErrFailedPrecondition,
			"namespace %q must be empty, but it still has %s",
			namespace, strings.Join(types, ", "),
		)
	}

	if err := bkt.DeleteBucket([]byte(namespace)); err != nil {
		if err == bolt.ErrBucketNotFound {
			return errors.Wrapf(errdefs.ErrNotFound, "namespace %q", namespace)
		}

		return err
	}

	return nil
}

// listNs returns the types of the remaining objects inside the given namespace.
// It doesn't return exact objects due to performance concerns.
func (s *namespaceStore) listNs(namespace string) ([]string, error) {
	var out []string

	if !isBucketEmpty(getImagesBucket(s.tx, namespace)) {
		out = append(out, "images")
	}
	if !isBucketEmpty(getBlobsBucket(s.tx, namespace)) {
		out = append(out, "blobs")
	}
	if !isBucketEmpty(getContainersBucket(s.tx, namespace)) {
		out = append(out, "containers")
	}

	if snbkt := getSnapshottersBucket(s.tx, namespace); snbkt != nil {
		if err := snbkt.ForEach(func(k, v []byte) error {
			if v == nil {
				if !isBucketEmpty(snbkt.Bucket(k)) {
					out = append(out, "snapshot-"+string(k))
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func isBucketEmpty(bkt *bolt.Bucket) bool {
	if bkt == nil {
		return true
	}

	k, _ := bkt.Cursor().First()
	return k == nil
}
