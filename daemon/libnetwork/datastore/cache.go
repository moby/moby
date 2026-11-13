package datastore

import (
	"errors"
	"fmt"
	"sync"

	store "github.com/moby/moby/v2/daemon/libnetwork/kvstore"
)

type kvMap map[string]KVObject

type cache struct {
	mu  sync.Mutex
	kmm map[string]kvMap
	ds  store.Store
	// index is the last index given to an object that the cache keeps
	// in place of the store. It only increases, so that a copy from
	// before a delete has an index that no object created later has.
	index uint64
}

func newCache(ds store.Store) *cache {
	return &cache{kmm: make(map[string]kvMap), ds: ds}
}

func (c *cache) kmap(kvObject KVObject) (kvMap, error) {
	c.mu.Lock()
	keyPrefix := Key(kvObject.KeyPrefix()...)
	kmap, ok := c.kmm[keyPrefix]
	c.mu.Unlock()

	if ok {
		return kmap, nil
	}

	kmap = kvMap{}

	kvList, err := c.ds.List(keyPrefix)
	if err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			// If the store doesn't have anything then there is nothing to
			// populate in the cache. Just bail out.
			goto out
		}

		return nil, fmt.Errorf("error while populating kmap: %v", err)
	}

	for _, kvPair := range kvList {
		// Ignore empty kvPair values
		if len(kvPair.Value) == 0 {
			continue
		}

		dstO := kvObject.New()
		if err := dstO.SetValue(kvPair.Value); err != nil {
			return nil, err
		}

		// Make sure the object has a correct view of the DB index in
		// case we need to modify it and update the DB.
		dstO.SetIndex(kvPair.LastIndex)

		kmap[Key(dstO.Key()...)] = dstO
	}

out:
	// There may multiple go routines racing to fill the
	// cache. The one which places the kmap in c.kmm first
	// wins. The others should just use what the first populated.
	c.mu.Lock()
	kmapNew, ok := c.kmm[keyPrefix]
	if ok {
		c.mu.Unlock()
		return kmapNew, nil
	}

	c.kmm[keyPrefix] = kmap
	c.mu.Unlock()

	return kmap, nil
}

func (c *cache) add(kvObject KVObject, atomic bool) error {
	kmap, err := c.kmap(kvObject)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	key := Key(kvObject.Key()...)
	// If atomic is true, cache needs to maintain its own index
	// for atomicity and the add needs to be atomic.
	if atomic {
		prev, ok := kmap[key]
		switch {
		case !kvObject.Exists() && ok:
			return ErrKeyModified
		case kvObject.Exists() && !ok:
			return ErrKeyNotFound
		case ok && prev.Index() != kvObject.Index():
			return ErrKeyModified
		}

		c.index++
		kvObject.SetIndex(c.index)
	}

	kmap[key] = kvObject
	return nil
}

func (c *cache) del(kvObject KVObject, atomic bool) error {
	kmap, err := c.kmap(kvObject)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	key := Key(kvObject.Key()...)
	// If atomic is true, cache needs to maintain its own index
	// for atomicity and del needs to be atomic.
	if atomic {
		prev, ok := kmap[key]
		if !ok {
			return ErrKeyNotFound
		}
		if prev.Index() != kvObject.Index() {
			return ErrKeyModified
		}
	}

	delete(kmap, key)
	return nil
}

func (c *cache) get(kvObject KVObject) error {
	kmap, err := c.kmap(kvObject)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	o, ok := kmap[Key(kvObject.Key()...)]
	if !ok {
		return ErrKeyNotFound
	}

	return o.CopyTo(kvObject)
}

func (c *cache) list(kvObject KVObject) ([]KVObject, error) {
	kmap, err := c.kmap(kvObject)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var kvol []KVObject
	for _, v := range kmap {
		kvol = append(kvol, v)
	}

	return kvol, nil
}
