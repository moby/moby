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
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/metadata/boltutil"
	"github.com/containerd/containerd/namespaces"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	bolt "go.etcd.io/bbolt"
)

type imageStore struct {
	db *DB
}

// NewImageStore returns a store backed by a bolt DB
func NewImageStore(db *DB) images.Store {
	return &imageStore{db: db}
}

func (s *imageStore) Get(ctx context.Context, name string) (images.Image, error) {
	var image images.Image

	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return images.Image{}, err
	}

	if err := view(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getImagesBucket(tx, namespace)
		if bkt == nil {
			return fmt.Errorf("image %q: %w", name, errdefs.ErrNotFound)
		}

		ibkt := bkt.Bucket([]byte(name))
		if ibkt == nil {
			return fmt.Errorf("image %q: %w", name, errdefs.ErrNotFound)
		}

		image.Name = name
		if err := readImage(&image, ibkt); err != nil {
			return fmt.Errorf("image %q: %w", name, err)
		}

		return nil
	}); err != nil {
		return images.Image{}, err
	}

	return image, nil
}

func (s *imageStore) List(ctx context.Context, fs ...string) ([]images.Image, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", err.Error(), errdefs.ErrInvalidArgument)
	}

	var m []images.Image
	if err := view(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getImagesBucket(tx, namespace)
		if bkt == nil {
			return nil // empty store
		}

		return bkt.ForEach(func(k, v []byte) error {
			var (
				image = images.Image{
					Name: string(k),
				}
				kbkt = bkt.Bucket(k)
			)

			if err := readImage(&image, kbkt); err != nil {
				return err
			}

			if filter.Match(adaptImage(image)) {
				m = append(m, image)
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}

	return m, nil
}

func (s *imageStore) Create(ctx context.Context, image images.Image) (images.Image, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return images.Image{}, err
	}

	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		if err := validateImage(&image); err != nil {
			return err
		}

		bkt, err := createImagesBucket(tx, namespace)
		if err != nil {
			return err
		}

		ibkt, err := bkt.CreateBucket([]byte(image.Name))
		if err != nil {
			if err != bolt.ErrBucketExists {
				return err
			}

			return fmt.Errorf("image %q: %w", image.Name, errdefs.ErrAlreadyExists)
		}

		image.CreatedAt = time.Now().UTC()
		image.UpdatedAt = image.CreatedAt
		return writeImage(ibkt, &image)
	}); err != nil {
		return images.Image{}, err
	}

	return image, nil
}

func (s *imageStore) Update(ctx context.Context, image images.Image, fieldpaths ...string) (images.Image, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return images.Image{}, err
	}

	if image.Name == "" {
		return images.Image{}, fmt.Errorf("image name is required for update: %w", errdefs.ErrInvalidArgument)
	}

	var updated images.Image

	if err := update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt, err := createImagesBucket(tx, namespace)
		if err != nil {
			return err
		}

		ibkt := bkt.Bucket([]byte(image.Name))
		if ibkt == nil {
			return fmt.Errorf("image %q: %w", image.Name, errdefs.ErrNotFound)
		}

		if err := readImage(&updated, ibkt); err != nil {
			return fmt.Errorf("image %q: %w", image.Name, err)
		}
		createdat := updated.CreatedAt
		updated.Name = image.Name

		if len(fieldpaths) > 0 {
			for _, path := range fieldpaths {
				if strings.HasPrefix(path, "labels.") {
					if updated.Labels == nil {
						updated.Labels = map[string]string{}
					}

					key := strings.TrimPrefix(path, "labels.")
					updated.Labels[key] = image.Labels[key]
					continue
				} else if strings.HasPrefix(path, "annotations.") {
					if updated.Target.Annotations == nil {
						updated.Target.Annotations = map[string]string{}
					}

					key := strings.TrimPrefix(path, "annotations.")
					updated.Target.Annotations[key] = image.Target.Annotations[key]
					continue
				}

				switch path {
				case "labels":
					updated.Labels = image.Labels
				case "target":
					// NOTE(stevvooe): While we allow setting individual labels, we
					// only support replacing the target as a unit, since that is
					// commonly pulled as a unit from other sources. It often doesn't
					// make sense to modify the size or digest without touching the
					// mediatype, as well, for example.
					updated.Target = image.Target
				case "annotations":
					updated.Target.Annotations = image.Target.Annotations
				default:
					return fmt.Errorf("cannot update %q field on image %q: %w", path, image.Name, errdefs.ErrInvalidArgument)
				}
			}
		} else {
			updated = image
		}

		if err := validateImage(&updated); err != nil {
			return err
		}

		updated.CreatedAt = createdat
		updated.UpdatedAt = time.Now().UTC()
		return writeImage(ibkt, &updated)
	}); err != nil {
		return images.Image{}, err
	}

	return updated, nil

}

func (s *imageStore) Delete(ctx context.Context, name string, opts ...images.DeleteOpt) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	return update(ctx, s.db, func(tx *bolt.Tx) error {
		bkt := getImagesBucket(tx, namespace)
		if bkt == nil {
			return fmt.Errorf("image %q: %w", name, errdefs.ErrNotFound)
		}

		if err = bkt.DeleteBucket([]byte(name)); err != nil {
			if err == bolt.ErrBucketNotFound {
				err = fmt.Errorf("image %q: %w", name, errdefs.ErrNotFound)
			}
			return err
		}

		atomic.AddUint32(&s.db.dirty, 1)

		return nil
	})
}

func validateImage(image *images.Image) error {
	if image.Name == "" {
		return fmt.Errorf("image name must not be empty: %w", errdefs.ErrInvalidArgument)
	}

	for k, v := range image.Labels {
		if err := labels.Validate(k, v); err != nil {
			return fmt.Errorf("image.Labels: %w", err)
		}
	}

	return validateTarget(&image.Target)
}

func validateTarget(target *ocispec.Descriptor) error {
	// NOTE(stevvooe): Only validate fields we actually store.

	if err := target.Digest.Validate(); err != nil {
		return fmt.Errorf("Target.Digest %q invalid: %v: %w", target.Digest, err, errdefs.ErrInvalidArgument)
	}

	if target.Size <= 0 {
		return fmt.Errorf("Target.Size must be greater than zero: %w", errdefs.ErrInvalidArgument)
	}

	if target.MediaType == "" {
		return fmt.Errorf("Target.MediaType must be set: %w", errdefs.ErrInvalidArgument)
	}

	return nil
}

func readImage(image *images.Image, bkt *bolt.Bucket) error {
	if err := boltutil.ReadTimestamps(bkt, &image.CreatedAt, &image.UpdatedAt); err != nil {
		return err
	}

	labels, err := boltutil.ReadLabels(bkt)
	if err != nil {
		return err
	}
	image.Labels = labels

	image.Target.Annotations, err = boltutil.ReadAnnotations(bkt)
	if err != nil {
		return err
	}

	tbkt := bkt.Bucket(bucketKeyTarget)
	if tbkt == nil {
		return errors.New("unable to read target bucket")
	}
	return tbkt.ForEach(func(k, v []byte) error {
		if v == nil {
			return nil // skip it? a bkt maybe?
		}

		// TODO(stevvooe): This is why we need to use byte values for
		// keys, rather than full arrays.
		switch string(k) {
		case string(bucketKeyDigest):
			image.Target.Digest = digest.Digest(v)
		case string(bucketKeyMediaType):
			image.Target.MediaType = string(v)
		case string(bucketKeySize):
			image.Target.Size, _ = binary.Varint(v)
		}

		return nil
	})
}

func writeImage(bkt *bolt.Bucket, image *images.Image) error {
	if err := boltutil.WriteTimestamps(bkt, image.CreatedAt, image.UpdatedAt); err != nil {
		return err
	}

	if err := boltutil.WriteLabels(bkt, image.Labels); err != nil {
		return fmt.Errorf("writing labels for image %v: %w", image.Name, err)
	}

	if err := boltutil.WriteAnnotations(bkt, image.Target.Annotations); err != nil {
		return fmt.Errorf("writing Annotations for image %v: %w", image.Name, err)
	}

	// write the target bucket
	tbkt, err := bkt.CreateBucketIfNotExists(bucketKeyTarget)
	if err != nil {
		return err
	}

	sizeEncoded, err := encodeInt(image.Target.Size)
	if err != nil {
		return err
	}

	for _, v := range [][2][]byte{
		{bucketKeyDigest, []byte(image.Target.Digest)},
		{bucketKeyMediaType, []byte(image.Target.MediaType)},
		{bucketKeySize, sizeEncoded},
	} {
		if err := tbkt.Put(v[0], v[1]); err != nil {
			return err
		}
	}

	return nil
}

func encodeInt(i int64) ([]byte, error) {
	var (
		buf      [binary.MaxVarintLen64]byte
		iEncoded = buf[:]
	)
	iEncoded = iEncoded[:binary.PutVarint(iEncoded, i)]

	if len(iEncoded) == 0 {
		return nil, fmt.Errorf("failed encoding integer = %v", i)
	}
	return iEncoded, nil
}
