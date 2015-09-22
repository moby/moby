package ipam

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

// Key provides the Key to be used in KV Store
func (a *Allocator) Key() []string {
	a.Lock()
	defer a.Unlock()
	return []string{dsConfigKey}
}

// KeyPrefix returns the immediate parent key that can be used for tree walk
func (a *Allocator) KeyPrefix() []string {
	a.Lock()
	defer a.Unlock()
	return []string{dsConfigKey}
}

// Value marshals the data to be stored in the KV store
func (a *Allocator) Value() []byte {
	a.Lock()
	defer a.Unlock()

	if a.subnets == nil {
		return []byte{}
	}
	m := map[string]interface{}{}
	for k, v := range a.subnets {
		m[k.String()] = v
	}

	b, err := json.Marshal(m)
	if err != nil {
		log.Warnf("Failed to marshal ipam configured subnets")
		return nil
	}
	return b
}

// SetValue unmarshalls the data from the KV store.
func (a *Allocator) SetValue(value []byte) error {
	var m map[string]*PoolData
	err := json.Unmarshal(value, &m)
	if err != nil {
		return err
	}
	for ks, d := range m {
		k := SubnetKey{}
		if err := k.FromString(ks); err != nil {
			return err
		}
		a.subnets[k] = d
	}
	return nil
}

// Index returns the latest DB Index as seen by this object
func (a *Allocator) Index() uint64 {
	a.Lock()
	defer a.Unlock()
	return a.dbIndex
}

// SetIndex method allows the datastore to store the latest DB Index into this object
func (a *Allocator) SetIndex(index uint64) {
	a.Lock()
	a.dbIndex = index
	a.dbExists = true
	a.Unlock()
}

// Exists method is true if this object has been stored in the DB.
func (a *Allocator) Exists() bool {
	a.Lock()
	defer a.Unlock()
	return a.dbExists
}

// Skip provides a way for a KV Object to avoid persisting it in the KV Store
func (a *Allocator) Skip() bool {
	return false
}

func (a *Allocator) watchForChanges() error {
	if a.store == nil {
		return nil
	}

	kvpChan, err := a.store.KVStore().Watch(datastore.Key(a.Key()...), nil)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case kvPair := <-kvpChan:
				if kvPair != nil {
					a.subnetConfigFromStore(kvPair)
				}
			}
		}
	}()
	return nil
}

func (a *Allocator) readFromStore() error {
	a.Lock()
	store := a.store
	a.Unlock()

	if store == nil {
		return nil
	}

	kvPair, err := a.store.KVStore().Get(datastore.Key(a.Key()...))
	if err != nil {
		return err
	}

	a.subnetConfigFromStore(kvPair)

	return nil
}

func (a *Allocator) writeToStore() error {
	a.Lock()
	store := a.store
	a.Unlock()
	if store == nil {
		return nil
	}
	err := store.PutObjectAtomic(a)
	if err == datastore.ErrKeyModified {
		return types.RetryErrorf("failed to perform atomic write (%v). retry might fix the error", err)
	}
	return err
}

func (a *Allocator) deleteFromStore() error {
	a.Lock()
	store := a.store
	a.Unlock()
	if store == nil {
		return nil
	}
	return store.DeleteObjectAtomic(a)
}

// DataScope method returns the storage scope of the datastore
func (a *Allocator) DataScope() datastore.DataScope {
	return datastore.GlobalScope
}
