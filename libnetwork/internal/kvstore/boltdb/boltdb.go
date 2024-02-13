package boltdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	store "github.com/docker/docker/libnetwork/internal/kvstore"
	bolt "go.etcd.io/bbolt"
)

var (
	// ErrBoltBucketOptionMissing is thrown when boltBcuket config option is missing
	ErrBoltBucketOptionMissing = errors.New("boltBucket config option missing")
)

const filePerm = 0o644

// BoltDB type implements the Store interface
type BoltDB struct {
	mu         sync.Mutex
	client     *bolt.DB
	boltBucket []byte
	dbIndex    uint64
	path       string
	timeout    time.Duration
}

const (
	libkvmetadatalen = 8
	transientTimeout = time.Duration(10) * time.Second
)

// New opens a new BoltDB connection to the specified path and bucket
func New(endpoint string, options *store.Config) (store.Store, error) {
	if (options == nil) || (len(options.Bucket) == 0) {
		return nil, ErrBoltBucketOptionMissing
	}

	dir, _ := filepath.Split(endpoint)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	db, err := bolt.Open(endpoint, filePerm, &bolt.Options{
		Timeout: options.ConnectionTimeout,
	})
	if err != nil {
		return nil, err
	}

	timeout := transientTimeout
	if options.ConnectionTimeout != 0 {
		timeout = options.ConnectionTimeout
	}

	b := &BoltDB{
		client:     db,
		path:       endpoint,
		boltBucket: []byte(options.Bucket),
		timeout:    timeout,
	}

	return b, nil
}

// Put the key, value pair. index number metadata is prepended to the value
func (b *BoltDB) Put(key string, value []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.client.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(b.boltBucket)
		if err != nil {
			return err
		}

		dbIndex := atomic.AddUint64(&b.dbIndex, 1)
		dbval := make([]byte, libkvmetadatalen)
		binary.LittleEndian.PutUint64(dbval, dbIndex)
		dbval = append(dbval, value...)

		return bucket.Put([]byte(key), dbval)
	})
}

// Exists checks if the key exists inside the store
func (b *BoltDB) Exists(key string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var exists bool
	err := b.client.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return store.ErrKeyNotFound
		}

		exists = len(bucket.Get([]byte(key))) > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	if !exists {
		return false, store.ErrKeyNotFound
	}
	return true, nil
}

// List returns the range of keys starting with the passed in prefix
func (b *BoltDB) List(keyPrefix string) ([]*store.KVPair, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var kv []*store.KVPair
	err := b.client.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return store.ErrKeyNotFound
		}

		cursor := bucket.Cursor()
		prefix := []byte(keyPrefix)

		for key, v := cursor.Seek(prefix); bytes.HasPrefix(key, prefix); key, v = cursor.Next() {
			dbIndex := binary.LittleEndian.Uint64(v[:libkvmetadatalen])
			v = v[libkvmetadatalen:]
			val := make([]byte, len(v))
			copy(val, v)

			kv = append(kv, &store.KVPair{
				Key:       string(key),
				Value:     val,
				LastIndex: dbIndex,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(kv) == 0 {
		return nil, store.ErrKeyNotFound
	}
	return kv, nil
}

// AtomicDelete deletes a value at "key" if the key
// has not been modified in the meantime, throws an
// error if this is the case
func (b *BoltDB) AtomicDelete(key string, previous *store.KVPair) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if previous == nil {
		return store.ErrPreviousNotSpecified
	}

	return b.client.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return store.ErrKeyNotFound
		}

		val := bucket.Get([]byte(key))
		if val == nil {
			return store.ErrKeyNotFound
		}
		dbIndex := binary.LittleEndian.Uint64(val[:libkvmetadatalen])
		if dbIndex != previous.LastIndex {
			return store.ErrKeyModified
		}
		return bucket.Delete([]byte(key))
	})
}

// AtomicPut puts a value at "key" if the key has not been
// modified since the last Put, throws an error if this is the case
func (b *BoltDB) AtomicPut(key string, value []byte, previous *store.KVPair) (*store.KVPair, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var dbIndex uint64
	dbval := make([]byte, libkvmetadatalen)
	err := b.client.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			if previous != nil {
				return store.ErrKeyNotFound
			}
			var err error
			bucket, err = tx.CreateBucket(b.boltBucket)
			if err != nil {
				return err
			}
		}
		// AtomicPut is equivalent to Put if previous is nil and the Ky
		// doesn't exist in the DB.
		val := bucket.Get([]byte(key))
		if previous == nil && len(val) != 0 {
			return store.ErrKeyExists
		}
		if previous != nil {
			if len(val) == 0 {
				return store.ErrKeyNotFound
			}
			dbIndex = binary.LittleEndian.Uint64(val[:libkvmetadatalen])
			if dbIndex != previous.LastIndex {
				return store.ErrKeyModified
			}
		}
		dbIndex = atomic.AddUint64(&b.dbIndex, 1)
		binary.LittleEndian.PutUint64(dbval, b.dbIndex)
		dbval = append(dbval, value...)
		return bucket.Put([]byte(key), dbval)
	})
	if err != nil {
		return nil, err
	}
	return &store.KVPair{Key: key, Value: value, LastIndex: dbIndex}, nil
}

// Close the db connection to the BoltDB
func (b *BoltDB) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.client.Close()
}
