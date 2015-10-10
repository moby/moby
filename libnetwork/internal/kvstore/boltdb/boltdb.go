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
)

const (
	filePerm os.FileMode = 0644
)

//BoltDB type implements the Store interface
type BoltDB struct {
	client     *bolt.DB
	boltBucket []byte
	dbIndex    uint64
	path       string
	timeout    time.Duration
	// By default libkv opens and closes the bolt DB connection  for every
	// get/put operation. This allows multiple apps to use a Bolt DB at the
	// same time.
	// PersistConnection flag provides an option to override ths behavior.
	// ie: open the connection in New and use it till Close is called.
	PersistConnection bool
	sync.Mutex
}

const (
	libkvmetadatalen = 8
	transientTimeout = time.Duration(10) * time.Second
)

// Register registers boltdb to libkv
func Register() {
	libkv.AddStore(store.BOLTDB, New)
}

// New opens a new BoltDB connection to the specified path and bucket
func New(endpoints []string, options *store.Config) (store.Store, error) {
	var (
		db          *bolt.DB
		err         error
		boltOptions *bolt.Options
	)

	if len(endpoints) > 1 {
		return nil, ErrMultipleEndpointsUnsupported
	}

	if (options == nil) || (len(options.Bucket) == 0) {
		return nil, ErrBoltBucketOptionMissing
	}

	dir, _ := filepath.Split(endpoints[0])
	if err = os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}

	if options.PersistConnection {
		boltOptions = &bolt.Options{Timeout: options.ConnectionTimeout}
		db, err = bolt.Open(endpoints[0], filePerm, boltOptions)
		if err != nil {
			return nil, err
		}
	}

	b := &BoltDB{
		client:            db,
		path:              endpoints[0],
		boltBucket:        []byte(options.Bucket),
		timeout:           transientTimeout,
		PersistConnection: options.PersistConnection,
	}

	return b, nil
}

func (b *BoltDB) reset() {
	b.path = ""
	b.boltBucket = []byte{}
}

func (b *BoltDB) getDBhandle() (*bolt.DB, error) {
	var (
		db  *bolt.DB
		err error
	)
	if !b.PersistConnection {
		boltOptions := &bolt.Options{Timeout: b.timeout}
		if db, err = bolt.Open(b.path, filePerm, boltOptions); err != nil {
			return nil, err
		}
		b.client = db
	}

	return b.client, nil
}

func (b *BoltDB) releaseDBhandle() {
	if !b.PersistConnection {
		b.client.Close()
	}
}

// Get the value at "key". BoltDB doesn't provide an inbuilt last modified index with every kv pair. Its implemented by
// by a atomic counter maintained by the libkv and appened to the value passed by the client.
func (b *BoltDB) Get(key string) (*store.KVPair, error) {
	var (
		val []byte
		db  *bolt.DB
		err error
	)
	b.Lock()
	defer b.Unlock()

	if db, err = b.getDBhandle(); err != nil {
		return nil, err
	}
	defer b.releaseDBhandle()

	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return ErrBoltBucketNotFound
		}

		v := bucket.Get([]byte(key))
		val = make([]byte, len(v))
		copy(val, v)

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
	var (
		dbIndex uint64
		db      *bolt.DB
		err     error
	)
	b.Lock()
	defer b.Unlock()

	dbval := make([]byte, libkvmetadatalen)

	if db, err = b.getDBhandle(); err != nil {
		return err
	}
	defer b.releaseDBhandle()

	err = db.Update(func(tx *bolt.Tx) error {
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
	var (
		db  *bolt.DB
		err error
	)
	b.Lock()
	defer b.Unlock()

	if db, err = b.getDBhandle(); err != nil {
		return err
	}
	defer b.releaseDBhandle()

	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return ErrBoltBucketNotFound
		}
		err := bucket.Delete([]byte(key))
		return err
	})
	return err
}

// Exists checks if the key exists inside the store
func (b *BoltDB) Exists(key string) (bool, error) {
	var (
		val []byte
		db  *bolt.DB
		err error
	)
	b.Lock()
	defer b.Unlock()

	if db, err = b.getDBhandle(); err != nil {
		return false, err
	}
	defer b.releaseDBhandle()

	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return ErrBoltBucketNotFound
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
	var (
		db  *bolt.DB
		err error
	)
	b.Lock()
	defer b.Unlock()

	kv := []*store.KVPair{}

	if db, err = b.getDBhandle(); err != nil {
		return nil, err
	}
	defer b.releaseDBhandle()

	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return ErrBoltBucketNotFound
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
	if len(kv) == 0 {
		return nil, store.ErrKeyNotFound
	}
	return kv, err
}

// AtomicDelete deletes a value at "key" if the key
// has not been modified in the meantime, throws an
// error if this is the case
func (b *BoltDB) AtomicDelete(key string, previous *store.KVPair) (bool, error) {
	var (
		val []byte
		db  *bolt.DB
		err error
	)
	b.Lock()
	defer b.Unlock()

	if previous == nil {
		return false, store.ErrPreviousNotSpecified
	}
	if db, err = b.getDBhandle(); err != nil {
		return false, err
	}
	defer b.releaseDBhandle()

	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return ErrBoltBucketNotFound
		}

		val = bucket.Get([]byte(key))
		if val == nil {
			return store.ErrKeyNotFound
		}
		dbIndex := binary.LittleEndian.Uint64(val[:libkvmetadatalen])
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
	var (
		val     []byte
		dbIndex uint64
		db      *bolt.DB
		err     error
	)
	b.Lock()
	defer b.Unlock()

	dbval := make([]byte, libkvmetadatalen)

	if db, err = b.getDBhandle(); err != nil {
		return false, nil, err
	}
	defer b.releaseDBhandle()

	err = db.Update(func(tx *bolt.Tx) error {
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
	b.Lock()
	defer b.Unlock()

	if !b.PersistConnection {
		b.reset()
	} else {
		b.client.Close()
	}
	return
}

// DeleteTree deletes a range of keys with a given prefix
func (b *BoltDB) DeleteTree(keyPrefix string) error {
	var (
		db  *bolt.DB
		err error
	)
	b.Lock()
	defer b.Unlock()

	if db, err = b.getDBhandle(); err != nil {
		return err
	}
	defer b.releaseDBhandle()

	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.boltBucket)
		if bucket == nil {
			return ErrBoltBucketNotFound
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
	return nil, store.ErrCallNotSupported
}

// Watch has to implemented at the library level since its not supported by BoltDB
func (b *BoltDB) Watch(key string, stopCh <-chan struct{}) (<-chan *store.KVPair, error) {
	return nil, store.ErrCallNotSupported
}

// WatchTree has to implemented at the library level since its not supported by BoltDB
func (b *BoltDB) WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	return nil, store.ErrCallNotSupported
}
