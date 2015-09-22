package boltdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/boltdb/bolt"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
)

var (
	// ErrMultipleEndpointsUnsupported is thrown when multiple endpoints specified for
	// BoltDB. Endpoint has to be a local file path
	ErrMultipleEndpointsUnsupported = errors.New("boltdb supports one endpoint and should be a file path")
	// ErrBoltBucketNotFound is thrown when specified BoltBD bucket doesn't exist in the DB
	ErrBoltBucketNotFound = errors.New("boltdb bucket doesn't exist")
	// ErrBoltBucketOptionMissing is thrown when boltBcuket config option is missing
	ErrBoltBucketOptionMissing = errors.New("boltBucket config option missing")
	// ErrBoltAPIUnsupported is thrown when an APIs unsupported by BoltDB backend is called
	ErrBoltAPIUnsupported = errors.New("API not supported by BoltDB backend")
)

//BoltDB type implements the Store interface
type BoltDB struct {
	client     *bolt.DB
	boltBucket []byte
	dbIndex    uint64
}

const (
	libkvmetadatalen = 8
)

// Register registers boltdb to libkv
func Register() {
	libkv.AddStore(store.BOLTDB, New)
}

// New opens a new BoltDB connection to the specified path and bucket
func New(endpoints []string, options *store.Config) (store.Store, error) {
	if len(endpoints) > 1 {
		return nil, ErrMultipleEndpointsUnsupported
	}

	if (options == nil) || (len(options.Bucket) == 0) {
		return nil, ErrBoltBucketOptionMissing
	}

	dir, _ := filepath.Split(endpoints[0])
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}

	var boltOptions *bolt.Options
	if options != nil {
		boltOptions = &bolt.Options{Timeout: options.ConnectionTimeout}
	}
	db, err := bolt.Open(endpoints[0], 0644, boltOptions)
	if err != nil {
		return nil, err
	}

	b := &BoltDB{}

	b.client = db
	b.boltBucket = []byte(options.Bucket)
	return b, nil
}

// Get the value at "key". BoltDB doesn't provide an inbuilt last modified index with every kv pair. Its implemented by
// by a atomic counter maintained by the libkv and appened to the value passed by the client.
func (b *BoltDB) Get(key string) (*store.KVPair, error) {
	var val []byte

	db := b.client
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return (ErrBoltBucketNotFound)
		}

		val = bucket.Get([]byte(key))

		return nil
	})

	if len(val) == 0 {
		return nil, store.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	dbIndex := binary.LittleEndian.Uint64(val[:libkvmetadatalen])
	val = val[libkvmetadatalen:]

	return &store.KVPair{Key: key, Value: val, LastIndex: (dbIndex)}, nil
}

//Put the key, value pair. index number metadata is prepended to the value
func (b *BoltDB) Put(key string, value []byte, opts *store.WriteOptions) error {
	var dbIndex uint64
	db := b.client
	dbval := make([]byte, libkvmetadatalen)

	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(b.boltBucket)
		if err != nil {
			return err
		}

		dbIndex = atomic.AddUint64(&b.dbIndex, 1)
		binary.LittleEndian.PutUint64(dbval, dbIndex)
		dbval = append(dbval, value...)

		err = bucket.Put([]byte(key), dbval)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

//Delete the value for the given key.
func (b *BoltDB) Delete(key string) error {
	db := b.client

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return (ErrBoltBucketNotFound)
		}
		err := bucket.Delete([]byte(key))
		return err
	})
	return err
}

// Exists checks if the key exists inside the store
func (b *BoltDB) Exists(key string) (bool, error) {
	var val []byte

	db := b.client
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return (ErrBoltBucketNotFound)
		}

		val = bucket.Get([]byte(key))

		return nil
	})

	if len(val) == 0 {
		return false, err
	}
	return true, err
}

// List returns the range of keys starting with the passed in prefix
func (b *BoltDB) List(keyPrefix string) ([]*store.KVPair, error) {
	kv := []*store.KVPair{}

	db := b.client
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return (ErrBoltBucketNotFound)
		}

		cursor := bucket.Cursor()
		prefix := []byte(keyPrefix)

		for key, val := cursor.Seek(prefix); bytes.HasPrefix(key, prefix); key, val = cursor.Next() {

			dbIndex := binary.LittleEndian.Uint64(val[:libkvmetadatalen])
			val = val[libkvmetadatalen:]

			kv = append(kv, &store.KVPair{
				Key:       string(key),
				Value:     val,
				LastIndex: dbIndex,
			})
		}
		return nil
	})
	if len(kv) == 0 {
		return nil, store.ErrKeyNotFound
	}
	return kv, err
}

// AtomicDelete deletes a value at "key" if the key
// has not been modified in the meantime, throws an
// error if this is the case
func (b *BoltDB) AtomicDelete(key string, previous *store.KVPair) (bool, error) {
	var val []byte
	var dbIndex uint64

	if previous == nil {
		return false, store.ErrPreviousNotSpecified
	}
	db := b.client

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return ErrBoltBucketNotFound
		}

		val = bucket.Get([]byte(key))
		dbIndex = binary.LittleEndian.Uint64(val[:libkvmetadatalen])
		if dbIndex != previous.LastIndex {
			return store.ErrKeyModified
		}
		err := bucket.Delete([]byte(key))
		return err
	})
	if err != nil {
		return false, err
	}
	return true, err
}

// AtomicPut puts a value at "key" if the key has not been
// modified since the last Put, throws an error if this is the case
func (b *BoltDB) AtomicPut(key string, value []byte, previous *store.KVPair, options *store.WriteOptions) (bool, *store.KVPair, error) {
	var val []byte
	var dbIndex uint64
	dbval := make([]byte, libkvmetadatalen)

	db := b.client

	err := db.Update(func(tx *bolt.Tx) error {
		var err error
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			if previous != nil {
				return ErrBoltBucketNotFound
			}
			bucket, err = tx.CreateBucket(b.boltBucket)
			if err != nil {
				return err
			}
		}
		// AtomicPut is equivalent to Put if previous is nil and the Ky
		// doesn't exist in the DB.
		val = bucket.Get([]byte(key))
		if previous == nil && len(val) != 0 {
			return store.ErrKeyModified
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
		return (bucket.Put([]byte(key), dbval))
	})
	if err != nil {
		return false, nil, err
	}

	updated := &store.KVPair{
		Key:       key,
		Value:     value,
		LastIndex: dbIndex,
	}

	return true, updated, nil
}

// Close the db connection to the BoltDB
func (b *BoltDB) Close() {
	db := b.client

	db.Close()
}

// DeleteTree deletes a range of keys with a given prefix
func (b *BoltDB) DeleteTree(keyPrefix string) error {
	db := b.client
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return (ErrBoltBucketNotFound)
		}

		cursor := bucket.Cursor()
		prefix := []byte(keyPrefix)

		for key, _ := cursor.Seek(prefix); bytes.HasPrefix(key, prefix); key, _ = cursor.Next() {
			_ = bucket.Delete([]byte(key))
		}
		return nil
	})

	return err
}

// NewLock has to implemented at the library level since its not supported by BoltDB
func (b *BoltDB) NewLock(key string, options *store.LockOptions) (store.Locker, error) {
	return nil, ErrBoltAPIUnsupported
}

// Watch has to implemented at the library level since its not supported by BoltDB
func (b *BoltDB) Watch(key string, stopCh <-chan struct{}) (<-chan *store.KVPair, error) {
	return nil, ErrBoltAPIUnsupported
}

// WatchTree has to implemented at the library level since its not supported by BoltDB
func (b *BoltDB) WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	return nil, ErrBoltAPIUnsupported
}
