package ipam

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

// Key provides the Key to be used in KV Store
func (cfg *PoolsConfig) Key() []string {
	cfg.Lock()
	defer cfg.Unlock()
	return []string{cfg.id}
}

// KeyPrefix returns the immediate parent key that can be used for tree walk
func (cfg *PoolsConfig) KeyPrefix() []string {
	cfg.Lock()
	defer cfg.Unlock()
	return []string{dsConfigKey}
}

// Value marshals the data to be stored in the KV store
func (cfg *PoolsConfig) Value() []byte {
	b, err := json.Marshal(cfg)
	if err != nil {
		log.Warnf("Failed to marshal ipam configured pools: %v", err)
		return nil
	}
	return b
}

// SetValue unmarshalls the data from the KV store.
func (cfg *PoolsConfig) SetValue(value []byte) error {
	rc := &PoolsConfig{subnets: make(map[SubnetKey]*PoolData)}
	if err := json.Unmarshal(value, rc); err != nil {
		return err
	}
	cfg.subnets = rc.subnets
	return nil
}

// Index returns the latest DB Index as seen by this object
func (cfg *PoolsConfig) Index() uint64 {
	cfg.Lock()
	defer cfg.Unlock()
	return cfg.dbIndex
}

// SetIndex method allows the datastore to store the latest DB Index into this object
func (cfg *PoolsConfig) SetIndex(index uint64) {
	cfg.Lock()
	cfg.dbIndex = index
	cfg.dbExists = true
	cfg.Unlock()
}

// Exists method is true if this object has been stored in the DB.
func (cfg *PoolsConfig) Exists() bool {
	cfg.Lock()
	defer cfg.Unlock()
	return cfg.dbExists
}

// Skip provides a way for a KV Object to avoid persisting it in the KV Store
func (cfg *PoolsConfig) Skip() bool {
	return false
}

func (cfg *PoolsConfig) watchForChanges() error {
	if cfg.ds == nil {
		return nil
	}
	kvpChan, err := cfg.ds.KVStore().Watch(datastore.Key(cfg.Key()...), nil)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case kvPair := <-kvpChan:
				if kvPair != nil {
					cfg.readFromKey(kvPair)
				}
			}
		}
	}()
	return nil
}

func (cfg *PoolsConfig) writeToStore() error {
	if cfg.ds == nil {
		return nil
	}
	err := cfg.ds.PutObjectAtomic(cfg)
	if err == datastore.ErrKeyModified {
		return types.RetryErrorf("failed to perform atomic write (%v). retry might fix the error", err)
	}
	return err
}

func (cfg *PoolsConfig) readFromStore() error {
	if cfg.ds == nil {
		return nil
	}
	return cfg.ds.GetObject(datastore.Key(cfg.Key()...), cfg)
}

func (cfg *PoolsConfig) readFromKey(kvPair *store.KVPair) {
	if cfg.dbIndex < kvPair.LastIndex {
		cfg.SetValue(kvPair.Value)
		cfg.dbIndex = kvPair.LastIndex
		cfg.dbExists = true
	}
}

func (cfg *PoolsConfig) deleteFromStore() error {
	if cfg.ds == nil {
		return nil
	}
	return cfg.ds.DeleteObjectAtomic(cfg)
}

// DataScope method returns the storage scope of the datastore
func (cfg *PoolsConfig) DataScope() datastore.DataScope {
	return cfg.scope
}
