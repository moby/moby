package identitycache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const bboltCacheBucket = "image-identity-cache-v1"

type boltBackend struct {
	db     *bolt.DB
	bucket []byte
}

// NewBoltDBBackend creates a bbolt-backed persistent cache backend.
func NewBoltDBBackend(root string) (Backend, error) {
	if root == "" {
		return NewNopBackend(), nil
	}
	cacheDir := filepath.Join(root, "image")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(filepath.Join(cacheDir, "identity-cache.db"), 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	b := &boltBackend{db: db, bucket: []byte(bboltCacheBucket)}
	if err := b.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(b.bucket)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return b, nil
}

func (b *boltBackend) Load(_ context.Context, cacheKey string, now time.Time) (Entry, bool, error) {
	var (
		entry   Entry
		payload []byte
	)
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.bucket)
		if bucket == nil {
			return nil
		}
		value := bucket.Get([]byte(cacheKey))
		if value == nil {
			return nil
		}
		payload = append([]byte(nil), value...)
		return nil
	})
	if err != nil {
		return Entry{}, false, err
	}
	if len(payload) == 0 {
		return Entry{}, false, nil
	}
	if err := json.Unmarshal(payload, &entry); err != nil {
		_ = b.delete(cacheKey)
		return Entry{}, false, nil
	}
	if now.After(entry.ExpiresAt) {
		if err := b.delete(cacheKey); err != nil {
			return Entry{}, false, err
		}
		return Entry{}, false, nil
	}
	return entry, true, nil
}

func (b *boltBackend) Store(_ context.Context, cacheKey string, entry Entry, now time.Time) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(b.bucket)
		if err != nil {
			return err
		}
		if err := bucket.Put([]byte(cacheKey), payload); err != nil {
			return err
		}
		return pruneExpiredBucketEntries(bucket, now)
	})
}

func (b *boltBackend) Close() error {
	if b == nil || b.db == nil {
		return nil
	}
	return b.db.Close()
}

func (b *boltBackend) delete(cacheKey string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.bucket)
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(cacheKey))
	})
}

func pruneExpiredBucketEntries(bucket *bolt.Bucket, now time.Time) error {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		var entry Entry
		if err := json.Unmarshal(value, &entry); err != nil || now.After(entry.ExpiresAt) {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
	}
	return nil
}
