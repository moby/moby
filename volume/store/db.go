package store

import (
	"encoding/json"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
)

var volumeBucketName = []byte("volumes")

type dbEntry struct {
	Key   []byte
	Value []byte
}

type volumeMetadata struct {
	Name    string
	Driver  string
	Labels  map[string]string
	Options map[string]string
}

func (s *VolumeStore) setMeta(name string, meta volumeMetadata) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return setMeta(tx, name, meta)
	})
}

func setMeta(tx *bolt.Tx, name string, meta volumeMetadata) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	b := tx.Bucket(volumeBucketName)
	return errors.Wrap(b.Put([]byte(name), metaJSON), "error setting volume metadata")
}

func (s *VolumeStore) getMeta(name string) (volumeMetadata, error) {
	var meta volumeMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		return getMeta(tx, name, &meta)
	})
	return meta, err
}

func getMeta(tx *bolt.Tx, name string, meta *volumeMetadata) error {
	b := tx.Bucket(volumeBucketName)
	val := b.Get([]byte(name))
	if string(val) == "" {
		return nil
	}
	if err := json.Unmarshal(val, meta); err != nil {
		return errors.Wrap(err, "error unmarshaling volume metadata")
	}
	return nil
}

func (s *VolumeStore) removeMeta(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return removeMeta(tx, name)
	})
}

func removeMeta(tx *bolt.Tx, name string) error {
	b := tx.Bucket(volumeBucketName)
	return errors.Wrap(b.Delete([]byte(name)), "error removing volume metadata")
}

func listEntries(tx *bolt.Tx) []*dbEntry {
	var entries []*dbEntry
	b := tx.Bucket(volumeBucketName)
	b.ForEach(func(k, v []byte) error {
		entries = append(entries, &dbEntry{k, v})
		return nil
	})
	return entries
}
