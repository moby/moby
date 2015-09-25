package bitseq

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

// Key provides the Key to be used in KV Store
func (h *Handle) Key() []string {
	h.Lock()
	defer h.Unlock()
	return []string{h.app, h.id}
}

// KeyPrefix returns the immediate parent key that can be used for tree walk
func (h *Handle) KeyPrefix() []string {
	h.Lock()
	defer h.Unlock()
	return []string{h.app}
}

// Value marshals the data to be stored in the KV store
func (h *Handle) Value() []byte {
	b, err := h.ToByteArray()
	if err != nil {
		log.Warnf("Failed to serialize Handle: %v", err)
		b = []byte{}
	}
	jv, err := json.Marshal(b)
	if err != nil {
		log.Warnf("Failed to json encode bitseq handler byte array: %v", err)
		return []byte{}
	}
	return jv
}

// SetValue unmarshals the data from the KV store
func (h *Handle) SetValue(value []byte) error {
	var b []byte
	if err := json.Unmarshal(value, &b); err != nil {
		return err
	}

	return h.FromByteArray(b)
}

// Index returns the latest DB Index as seen by this object
func (h *Handle) Index() uint64 {
	h.Lock()
	defer h.Unlock()
	return h.dbIndex
}

// SetIndex method allows the datastore to store the latest DB Index into this object
func (h *Handle) SetIndex(index uint64) {
	h.Lock()
	h.dbIndex = index
	h.dbExists = true
	h.Unlock()
}

// Exists method is true if this object has been stored in the DB.
func (h *Handle) Exists() bool {
	h.Lock()
	defer h.Unlock()
	return h.dbExists
}

// Skip provides a way for a KV Object to avoid persisting it in the KV Store
func (h *Handle) Skip() bool {
	return false
}

// DataScope method returns the storage scope of the datastore
func (h *Handle) DataScope() datastore.DataScope {
	return datastore.GlobalScope
}

func (h *Handle) watchForChanges() error {
	h.Lock()
	store := h.store
	h.Unlock()

	if store == nil {
		return nil
	}

	kvpChan, err := store.KVStore().Watch(datastore.Key(h.Key()...), nil)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case kvPair := <-kvpChan:
				// Only process remote update
				if kvPair != nil && (kvPair.LastIndex != h.Index()) {
					err := h.fromDsValue(kvPair.Value)
					if err != nil {
						log.Warnf("Failed to reconstruct bitseq handle from ds watch: %s", err.Error())
					} else {
						h.SetIndex(kvPair.LastIndex)
					}
				}
			}
		}
	}()
	return nil
}

func (h *Handle) fromDsValue(value []byte) error {
	var ba []byte
	if err := json.Unmarshal(value, &ba); err != nil {
		return fmt.Errorf("failed to decode json: %s", err.Error())
	}
	if err := h.FromByteArray(ba); err != nil {
		return fmt.Errorf("failed to decode handle: %s", err.Error())
	}
	return nil
}

func (h *Handle) writeToStore() error {
	h.Lock()
	store := h.store
	h.Unlock()
	if store == nil {
		return nil
	}
	err := store.PutObjectAtomic(h)
	if err == datastore.ErrKeyModified {
		return types.RetryErrorf("failed to perform atomic write (%v). Retry might fix the error", err)
	}
	return err
}

func (h *Handle) deleteFromStore() error {
	h.Lock()
	store := h.store
	h.Unlock()
	if store == nil {
		return nil
	}
	return store.DeleteObjectAtomic(h)
}
