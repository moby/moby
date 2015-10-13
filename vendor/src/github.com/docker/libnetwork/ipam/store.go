package ipam

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

// Key provides the Key to be used in KV Store
func (aSpace *addrSpace) Key() []string {
	aSpace.Lock()
	defer aSpace.Unlock()
	return []string{aSpace.id}
}

// KeyPrefix returns the immediate parent key that can be used for tree walk
func (aSpace *addrSpace) KeyPrefix() []string {
	aSpace.Lock()
	defer aSpace.Unlock()
	return []string{dsConfigKey}
}

// Value marshals the data to be stored in the KV store
func (aSpace *addrSpace) Value() []byte {
	b, err := json.Marshal(aSpace)
	if err != nil {
		log.Warnf("Failed to marshal ipam configured pools: %v", err)
		return nil
	}
	return b
}

// SetValue unmarshalls the data from the KV store.
func (aSpace *addrSpace) SetValue(value []byte) error {
	rc := &addrSpace{subnets: make(map[SubnetKey]*PoolData)}
	if err := json.Unmarshal(value, rc); err != nil {
		return err
	}
	aSpace.subnets = rc.subnets
	return nil
}

// Index returns the latest DB Index as seen by this object
func (aSpace *addrSpace) Index() uint64 {
	aSpace.Lock()
	defer aSpace.Unlock()
	return aSpace.dbIndex
}

// SetIndex method allows the datastore to store the latest DB Index into this object
func (aSpace *addrSpace) SetIndex(index uint64) {
	aSpace.Lock()
	aSpace.dbIndex = index
	aSpace.dbExists = true
	aSpace.Unlock()
}

// Exists method is true if this object has been stored in the DB.
func (aSpace *addrSpace) Exists() bool {
	aSpace.Lock()
	defer aSpace.Unlock()
	return aSpace.dbExists
}

// Skip provides a way for a KV Object to avoid persisting it in the KV Store
func (aSpace *addrSpace) Skip() bool {
	return false
}

func (a *Allocator) getStore(as string) datastore.DataStore {
	a.Lock()
	defer a.Unlock()

	if aSpace, ok := a.addrSpaces[as]; ok {
		return aSpace.ds
	}

	return nil
}

func (a *Allocator) getAddressSpaceFromStore(as string) (*addrSpace, error) {
	store := a.getStore(as)
	if store == nil {
		return nil, fmt.Errorf("store for address space %s not found", as)
	}

	pc := &addrSpace{id: dsConfigKey + "/" + as, ds: store, alloc: a}
	if err := store.GetObject(datastore.Key(pc.Key()...), pc); err != nil {
		if err == datastore.ErrKeyNotFound {
			return nil, nil
		}

		return nil, fmt.Errorf("could not get pools config from store: %v", err)
	}

	return pc, nil
}

func (a *Allocator) writeToStore(aSpace *addrSpace) error {
	store := aSpace.store()
	if store == nil {
		return fmt.Errorf("invalid store while trying to write %s address space", aSpace.DataScope())
	}

	err := store.PutObjectAtomic(aSpace)
	if err == datastore.ErrKeyModified {
		return types.RetryErrorf("failed to perform atomic write (%v). retry might fix the error", err)
	}

	return err
}

func (a *Allocator) deleteFromStore(aSpace *addrSpace) error {
	store := aSpace.store()
	if store == nil {
		return fmt.Errorf("invalid store while trying to delete %s address space", aSpace.DataScope())
	}

	return store.DeleteObjectAtomic(aSpace)
}

// DataScope method returns the storage scope of the datastore
func (aSpace *addrSpace) DataScope() string {
	aSpace.Lock()
	defer aSpace.Unlock()

	return aSpace.scope
}
