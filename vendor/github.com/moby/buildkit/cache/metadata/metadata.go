package metadata

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	mainBucket     = "_main"
	indexBucket    = "_index"
	externalBucket = "_external"
)

var errNotFound = errors.Errorf("not found")

type Store struct {
	db *bolt.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open database file %s", dbPath)
	}
	return &Store{db: db}, nil
}

func (s *Store) DB() *bolt.DB {
	return s.db
}

func (s *Store) All() ([]*StorageItem, error) {
	var out []*StorageItem
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(mainBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(key, _ []byte) error {
			b := b.Bucket(key)
			if b == nil {
				return nil
			}
			si, err := newStorageItem(string(key), b, s)
			if err != nil {
				return err
			}
			out = append(out, si)
			return nil
		})
	})
	return out, errors.WithStack(err)
}

func (s *Store) Probe(index string) (bool, error) {
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(indexBucket))
		if b == nil {
			return nil
		}
		main := tx.Bucket([]byte(mainBucket))
		if main == nil {
			return nil
		}
		search := []byte(indexKey(index, ""))
		c := b.Cursor()
		k, _ := c.Seek(search)
		if k != nil && bytes.HasPrefix(k, search) {
			exists = true
		}
		return nil
	})
	return exists, errors.WithStack(err)
}

func (s *Store) Search(index string) ([]*StorageItem, error) {
	var out []*StorageItem
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(indexBucket))
		if b == nil {
			return nil
		}
		main := tx.Bucket([]byte(mainBucket))
		if main == nil {
			return nil
		}
		index = indexKey(index, "")
		c := b.Cursor()
		k, _ := c.Seek([]byte(index))
		for {
			if k != nil && strings.HasPrefix(string(k), index) {
				itemID := strings.TrimPrefix(string(k), index)
				k, _ = c.Next()
				b := main.Bucket([]byte(itemID))
				if b == nil {
					logrus.Errorf("index pointing to missing record %s", itemID)
					continue
				}
				si, err := newStorageItem(itemID, b, s)
				if err != nil {
					return err
				}
				out = append(out, si)
			} else {
				break
			}
		}
		return nil
	})
	return out, errors.WithStack(err)
}

func (s *Store) View(id string, fn func(b *bolt.Bucket) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(mainBucket))
		if b == nil {
			return errors.WithStack(errNotFound)
		}
		b = b.Bucket([]byte(id))
		if b == nil {
			return errors.WithStack(errNotFound)
		}
		return fn(b)
	})
}

func (s *Store) Clear(id string) error {
	return errors.WithStack(s.db.Update(func(tx *bolt.Tx) error {
		external := tx.Bucket([]byte(externalBucket))
		if external != nil {
			external.DeleteBucket([]byte(id))
		}
		main := tx.Bucket([]byte(mainBucket))
		if main == nil {
			return nil
		}
		b := main.Bucket([]byte(id))
		if b == nil {
			return nil
		}
		si, err := newStorageItem(id, b, s)
		if err != nil {
			return err
		}
		if indexes := si.Indexes(); len(indexes) > 0 {
			b := tx.Bucket([]byte(indexBucket))
			if b != nil {
				for _, index := range indexes {
					if err := b.Delete([]byte(indexKey(index, id))); err != nil {
						return err
					}
				}
			}
		}
		return main.DeleteBucket([]byte(id))
	}))
}

func (s *Store) Update(id string, fn func(b *bolt.Bucket) error) error {
	return errors.WithStack(s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(mainBucket))
		if err != nil {
			return errors.WithStack(err)
		}
		b, err = b.CreateBucketIfNotExists([]byte(id))
		if err != nil {
			return errors.WithStack(err)
		}
		return fn(b)
	}))
}

func (s *Store) Get(id string) (*StorageItem, bool) {
	empty := func() *StorageItem {
		si, _ := newStorageItem(id, nil, s)
		return si
	}
	tx, err := s.db.Begin(false)
	if err != nil {
		return empty(), false
	}
	defer tx.Rollback()
	b := tx.Bucket([]byte(mainBucket))
	if b == nil {
		return empty(), false
	}
	b = b.Bucket([]byte(id))
	if b == nil {
		return empty(), false
	}
	si, _ := newStorageItem(id, b, s)
	return si, true
}

func (s *Store) Close() error {
	return errors.WithStack(s.db.Close())
}

type StorageItem struct {
	id      string
	vmu     sync.RWMutex
	values  map[string]*Value
	qmu     sync.Mutex
	queue   []func(*bolt.Bucket) error
	storage *Store
}

func newStorageItem(id string, b *bolt.Bucket, s *Store) (*StorageItem, error) {
	si := &StorageItem{
		id:      id,
		storage: s,
		values:  make(map[string]*Value),
	}
	if b != nil {
		if err := b.ForEach(func(k, v []byte) error {
			var sv Value
			if len(v) > 0 {
				if err := json.Unmarshal(v, &sv); err != nil {
					return errors.WithStack(err)
				}
				si.values[string(k)] = &sv
			}
			return nil
		}); err != nil {
			return si, errors.WithStack(err)
		}
	}
	return si, nil
}

func (s *StorageItem) Storage() *Store {
	return s.storage
}

func (s *StorageItem) ID() string {
	return s.id
}

func (s *StorageItem) Update(fn func(b *bolt.Bucket) error) error {
	return s.storage.Update(s.id, fn)
}

func (s *StorageItem) Metadata() *StorageItem {
	return s
}

func (s *StorageItem) Keys() []string {
	s.vmu.RLock()
	keys := make([]string, 0, len(s.values))
	for k := range s.values {
		keys = append(keys, k)
	}
	s.vmu.RUnlock()
	return keys
}

func (s *StorageItem) Get(k string) *Value {
	s.vmu.RLock()
	v := s.values[k]
	s.vmu.RUnlock()
	return v
}

func (s *StorageItem) GetExternal(k string) ([]byte, error) {
	var dt []byte
	err := s.storage.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(externalBucket))
		if b == nil {
			return errors.WithStack(errNotFound)
		}
		b = b.Bucket([]byte(s.id))
		if b == nil {
			return errors.WithStack(errNotFound)
		}
		dt2 := b.Get([]byte(k))
		if dt2 == nil {
			return errors.WithStack(errNotFound)
		}
		// data needs to be copied as boltdb can reuse the buffer after View returns
		dt = make([]byte, len(dt2))
		copy(dt, dt2)
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return dt, nil
}

func (s *StorageItem) SetExternal(k string, dt []byte) error {
	return errors.WithStack(s.storage.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(externalBucket))
		if err != nil {
			return errors.WithStack(err)
		}
		b, err = b.CreateBucketIfNotExists([]byte(s.id))
		if err != nil {
			return errors.WithStack(err)
		}
		return b.Put([]byte(k), dt)
	}))
}

func (s *StorageItem) Queue(fn func(b *bolt.Bucket) error) {
	s.qmu.Lock()
	defer s.qmu.Unlock()
	s.queue = append(s.queue, fn)
}

func (s *StorageItem) Commit() error {
	s.qmu.Lock()
	defer s.qmu.Unlock()
	return errors.WithStack(s.Update(func(b *bolt.Bucket) error {
		for _, fn := range s.queue {
			if err := fn(b); err != nil {
				return errors.WithStack(err)
			}
		}
		s.queue = s.queue[:0]
		return nil
	}))
}

func (s *StorageItem) Indexes() (out []string) {
	s.vmu.RLock()
	for _, v := range s.values {
		if v.Index != "" {
			out = append(out, v.Index)
		}
	}
	s.vmu.RUnlock()
	return
}

func (s *StorageItem) SetValue(b *bolt.Bucket, key string, v *Value) error {
	s.vmu.Lock()
	defer s.vmu.Unlock()
	return s.setValue(b, key, v)
}

func (s *StorageItem) ClearIndex(tx *bolt.Tx, index string) error {
	s.vmu.Lock()
	defer s.vmu.Unlock()
	return s.clearIndex(tx, index)
}

func (s *StorageItem) clearIndex(tx *bolt.Tx, index string) error {
	b, err := tx.CreateBucketIfNotExists([]byte(indexBucket))
	if err != nil {
		return errors.WithStack(err)
	}
	return b.Delete([]byte(indexKey(index, s.ID())))
}

func (s *StorageItem) setValue(b *bolt.Bucket, key string, v *Value) error {
	if v == nil {
		if old, ok := s.values[key]; ok {
			if old.Index != "" {
				s.clearIndex(b.Tx(), old.Index) // ignore error
			}
		}
		if err := b.Put([]byte(key), nil); err != nil {
			return err
		}
		delete(s.values, key)
		return nil
	}
	dt, err := json.Marshal(v)
	if err != nil {
		return errors.WithStack(err)
	}
	if err := b.Put([]byte(key), dt); err != nil {
		return errors.WithStack(err)
	}
	if v.Index != "" {
		b, err := b.Tx().CreateBucketIfNotExists([]byte(indexBucket))
		if err != nil {
			return errors.WithStack(err)
		}
		if err := b.Put([]byte(indexKey(v.Index, s.ID())), []byte{}); err != nil {
			return errors.WithStack(err)
		}
	}
	s.values[key] = v
	return nil
}

var ErrSkipSetValue = errors.New("skip setting metadata value")

func (s *StorageItem) GetAndSetValue(key string, fn func(*Value) (*Value, error)) error {
	return s.Update(func(b *bolt.Bucket) error {
		s.vmu.Lock()
		defer s.vmu.Unlock()
		v, err := fn(s.values[key])
		if errors.Is(err, ErrSkipSetValue) {
			return nil
		} else if err != nil {
			return err
		}
		return s.setValue(b, key, v)
	})
}

type Value struct {
	Value json.RawMessage `json:"value,omitempty"`
	Index string          `json:"index,omitempty"`
}

func NewValue(v interface{}) (*Value, error) {
	dt, err := json.Marshal(v)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &Value{Value: json.RawMessage(dt)}, nil
}

func (v *Value) Unmarshal(target interface{}) error {
	return errors.WithStack(json.Unmarshal(v.Value, target))
}

func indexKey(index, target string) string {
	return index + "::" + target
}
