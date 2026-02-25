package identitycache

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	boltdb "github.com/moby/buildkit/util/db"
	"github.com/moby/buildkit/util/db/boltutil"
	bolt "go.etcd.io/bbolt"
)

var bboltCacheBucket = []byte("image-identity-cache-v1")

const (
	// 15m matches the shortest cache TTL (imageIdentityErrorCacheTTL), so
	// prune won't lag far behind shortest-lived entries.
	pruneIntervalMin    = 15 * time.Minute
	pruneIntervalSpread = 15 * time.Minute
)

type boltBackend struct {
	db        boltdb.DB
	closeOnce sync.Once
	closeErr  error
	stopPrune chan struct{}
	pruneDone chan struct{}
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
	db, err := boltutil.SafeOpen(filepath.Join(cacheDir, "identity-cache.db"), 0o600, nil)
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
	b.startPrune()
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
		if err := b.delete(cacheKey); err != nil {
			return Entry{}, false, err
		}
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

func (b *boltBackend) Close() error {
	if b == nil || b.db == nil {
		return nil
	}
	b.closeOnce.Do(func() {
		if b.stopPrune != nil {
			close(b.stopPrune)
		}
		if b.pruneDone != nil {
			<-b.pruneDone
		}
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

func (b *boltBackend) startPrune() {
	b.stopPrune = make(chan struct{})
	b.pruneDone = make(chan struct{})
	go func() {
		defer close(b.pruneDone)
		timer := time.NewTimer(nextPruneDelay())
		defer timer.Stop()
		for {
			select {
			case <-b.stopPrune:
				return
			case <-timer.C:
				_ = b.pruneExpiredEntries(time.Now().UTC())
				timer.Reset(nextPruneDelay())
			}
		}
	}()
}

func nextPruneDelay() time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(pruneIntervalSpread)))
	if err != nil {
		return pruneIntervalMin
	}
	return pruneIntervalMin + time.Duration(n.Int64())
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
