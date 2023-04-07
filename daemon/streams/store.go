package streams

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/streams"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/go-digest"
	"go.etcd.io/bbolt"
)

var (
	bucketName  = []byte("streams")
	errNotFound = errdefs.NotFound(errors.New("stream not found"))
)

func NewStore(db *bbolt.DB) *Store {
	return &Store{db: db}
}

type Store struct {
	db  *bbolt.DB
	dir string
}

func (s *Store) Create(stream streams.Stream) error {
	if stream.ID == "" {
		return errdefs.InvalidParameter(errors.New("stream ID cannot be empty"))
	}

	data, err := json.Marshal(stream)
	if err != nil {
		return errdefs.System(err)
	}

	id := []byte(stream.ID)
	err = s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			if errors.Is(err, bbolt.ErrBucketNameRequired) {
				return errdefs.InvalidParameter(err)
			}
			return errdefs.System(err)
		}
		if bucket.Get(id) != nil {
			return errdefs.Conflict(fmt.Errorf("stream already exists: %s", stream.ID))
		}
		return bucket.Put(id, data)
	})
	return err
}

func (s *Store) Get(key string) (*streams.Stream, error) {
	k := []byte(key)

	var v []byte

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errNotFound
		}
		v = bucket.Get(k)
		return nil
	})
	if err != nil {
		return nil, errdefs.System(err)
	}

	if v == nil {
		return nil, errNotFound
	}

	var stream streams.Stream
	if err := json.Unmarshal(v, &stream); err != nil {
		return nil, errdefs.System(err)
	}

	return &stream, nil
}

func (s *Store) Delete(key string) error {
	k := []byte(key)
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}

		dgst := digest.FromBytes(k)
		if err := os.Remove(filepath.Join(s.dir, dgst.String())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errdefs.System(err)
		}
		return bucket.Delete(k)
	})
}

func (s *Store) Close() error {
	return s.db.Close()
}
