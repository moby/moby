package identitycache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/containerd/log"
	bolt "go.etcd.io/bbolt"
)

var bboltCacheBucket = []byte("image-identity-cache-v1")

type boltBackend struct {
	db        *bolt.DB
	closeOnce sync.Once
	closeErr  error
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
	db, err := safeOpen(filepath.Join(cacheDir, "identity-cache.db"), 0o600, nil)
	if err != nil {
		return nil, err
	}
	b := &boltBackend{db: db}
	if err := b.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bboltCacheBucket)
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
		bucket := tx.Bucket(bboltCacheBucket)
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
		return Entry{}, false, nil
	}
	return entry, true, nil
}

func (b *boltBackend) Store(_ context.Context, cacheKey string, entry Entry, _ time.Time) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bboltCacheBucket)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(cacheKey), payload)
	})
}

func (b *boltBackend) Walk(ctx context.Context, _ time.Time, fn func(cacheKey string, entry Entry) error) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bboltCacheBucket)
		if bucket == nil {
			return nil
		}
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}

			var entry Entry
			if err := json.Unmarshal(value, &entry); err != nil {
				continue
			}
			if err := fn(string(key), entry); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *boltBackend) PruneExpired(_ context.Context, now time.Time) error {
	return b.pruneExpiredEntries(now)
}

func (b *boltBackend) Close() error {
	if b == nil || b.db == nil {
		return nil
	}
	b.closeOnce.Do(func() {
		b.closeErr = b.db.Close()
	})
	return b.closeErr
}

func (b *boltBackend) delete(cacheKey string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bboltCacheBucket)
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(cacheKey))
	})
}

func (b *boltBackend) pruneExpiredEntries(now time.Time) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bboltCacheBucket)
		if bucket == nil {
			return nil
		}
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var entry Entry
			if err := json.Unmarshal(value, &entry); err != nil || now.After(entry.ExpiresAt) {
				if err := cursor.Delete(); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// safeOpen opens a bolt database with automatic recovery from corruption.
// If the database file is corrupted, it backs up the corrupted file and creates
// a new empty database.
func safeOpen(dbPath string, mode os.FileMode, opts *bolt.Options) (db *bolt.DB, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
		// If we fail opening the DB, but can read a non-empty file, try resetting it.
		if err != nil && fileHasContent(dbPath) {
			db, err = fallbackOpen(dbPath, mode, opts, err)
		}
	}()
	return openDB(dbPath, mode, opts)
}

func openDB(dbPath string, mode os.FileMode, opts *bolt.Options) (*bolt.DB, error) {
	bdb, err := bolt.Open(dbPath, mode, opts)
	if err != nil {
		return nil, err
	}
	return bdb, nil
}

func fallbackOpen(dbPath string, mode os.FileMode, opts *bolt.Options, openErr error) (*bolt.DB, error) {
	backupPath := dbPath + "." + fmt.Sprintf("%d", time.Now().UnixNano()) + ".bak"
	log.L.Errorf("failed to open moby image identity cache database %s, resetting to empty. "+
		"Old database is backed up to %s. This usually means dockerd crashed or was terminated abruptly, leaving the cache DB corrupted. "+
		"If this keeps happening, please report at https://github.com/moby/moby/issues. original error: %v",
		dbPath, backupPath, openErr)
	if err := os.Rename(dbPath, backupPath); err != nil {
		return nil, fmt.Errorf("failed to rename database file %s to %s: %w", dbPath, backupPath, err)
	}
	// Second open should create a new database; failure here is permanent.
	return openDB(dbPath, mode, opts)
}

func fileHasContent(dbPath string) bool {
	st, err := os.Stat(dbPath)
	return err == nil && st.Size() > 0
}
