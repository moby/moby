package bitseq

import (
	"encoding/json"

	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/types"
)

// Key provides the Key to be used in KV Store
func (h *Handle) Key() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return []string{h.app, h.id}
}

// KeyPrefix returns the immediate parent key that can be used for tree walk
func (h *Handle) KeyPrefix() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return []string{h.app}
}

// Value marshals the data to be stored in the KV store
func (h *Handle) Value() []byte {
	b, err := json.Marshal(h)
	if err != nil {
		return nil
	}
	return b
}

// SetValue unmarshals the data from the KV store
func (h *Handle) SetValue(value []byte) error {
	return json.Unmarshal(value, h)
}

// Index returns the latest DB Index as seen by this object
func (h *Handle) Index() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dbIndex
}

// SetIndex method allows the datastore to store the latest DB Index into this object
func (h *Handle) SetIndex(index uint64) {
	h.mu.Lock()
	h.dbIndex = index
	h.dbExists = true
	h.mu.Unlock()
}

// Exists method is true if this object has been stored in the DB.
func (h *Handle) Exists() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dbExists
}

// New method returns a handle based on the receiver handle
func (h *Handle) New() datastore.KVObject {
	h.mu.Lock()
	defer h.mu.Unlock()

	return &Handle{
		app:   h.app,
		store: h.store,
	}
}

// CopyTo deep copies the handle into the passed destination object
func (h *Handle) CopyTo(o datastore.KVObject) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	dstH := o.(*Handle)
	if h == dstH {
		return nil
	}
	dstH.mu.Lock()
	defer dstH.mu.Unlock()
	dstH.bm = bitmap.Copy(h.bm)
	dstH.app = h.app
	dstH.id = h.id
	dstH.dbIndex = h.dbIndex
	dstH.dbExists = h.dbExists
	dstH.store = h.store

	return nil
}

// Skip provides a way for a KV Object to avoid persisting it in the KV Store
func (h *Handle) Skip() bool {
	return false
}

// DataScope method returns the storage scope of the datastore
func (h *Handle) DataScope() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.store.Scope()
}

func (h *Handle) writeToStore() error {
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
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
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return nil
	}
	return store.DeleteObjectAtomic(h)
}
