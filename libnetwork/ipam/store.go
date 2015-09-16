package ipam

import (
	"encoding/json"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

// Key provides the Key to be used in KV Store
func (a *Allocator) Key() []string {
	a.Lock()
	defer a.Unlock()
	return []string{a.App, a.ID}
}

// KeyPrefix returns the immediate parent key that can be used for tree walk
func (a *Allocator) KeyPrefix() []string {
	a.Lock()
	defer a.Unlock()
	return []string{a.App}
}

// Value marshals the data to be stored in the KV store
func (a *Allocator) Value() []byte {
	a.Lock()
	defer a.Unlock()

	if a.subnets == nil {
		return []byte{}
	}

	b, err := subnetsToByteArray(a.subnets)
	if err != nil {
		return nil
	}
	return b
}

// SetValue unmarshalls the data from the KV store.
func (a *Allocator) SetValue(value []byte) error {
	a.subnets = byteArrayToSubnets(value)
	return nil
}

func subnetsToByteArray(m map[subnetKey]*SubnetInfo) ([]byte, error) {
	if m == nil {
		return nil, nil
	}

	mm := make(map[string]string, len(m))
	for k, v := range m {
		mm[k.String()] = v.Subnet.String()
	}

	return json.Marshal(mm)
}

func byteArrayToSubnets(ba []byte) map[subnetKey]*SubnetInfo {
	m := map[subnetKey]*SubnetInfo{}

	if ba == nil || len(ba) == 0 {
		return m
	}

	var mm map[string]string
	err := json.Unmarshal(ba, &mm)
	if err != nil {
		log.Warnf("Failed to decode subnets byte array: %v", err)
		return m
	}
	for ks, vs := range mm {
		sk := subnetKey{}
		if err := sk.FromString(ks); err != nil {
			log.Warnf("Failed to decode subnets map entry: (%s, %s)", ks, vs)
			continue
		}
		si := &SubnetInfo{}
		_, nw, err := net.ParseCIDR(vs)
		if err != nil {
			log.Warnf("Failed to decode subnets map entry value: (%s, %s)", ks, vs)
			continue
		}
		si.Subnet = nw
		m[sk] = si
	}
	return m
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
					log.Debugf("Got notification for key %v: %v", kvPair.Key, kvPair.Value)
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
