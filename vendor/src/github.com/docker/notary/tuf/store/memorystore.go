package store

import (
	"crypto/sha256"
	"fmt"

	"github.com/docker/notary"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/utils"
)

// NewMemoryStore returns a MetadataStore that operates entirely in memory.
// Very useful for testing
func NewMemoryStore(meta map[string][]byte) *MemoryStore {
	var consistent = make(map[string][]byte)
	if meta == nil {
		meta = make(map[string][]byte)
	} else {
		// add all seed meta to consistent
		for name, data := range meta {
			checksum := sha256.Sum256(data)
			path := utils.ConsistentName(name, checksum[:])
			consistent[path] = data
		}
	}
	return &MemoryStore{
		meta:       meta,
		consistent: consistent,
		keys:       make(map[string][]data.PrivateKey),
	}
}

// MemoryStore implements a mock RemoteStore entirely in memory.
// For testing purposes only.
type MemoryStore struct {
	meta       map[string][]byte
	consistent map[string][]byte
	keys       map[string][]data.PrivateKey
}

// GetMeta returns up to size bytes of data references by name.
// If size is -1, this corresponds to "infinite," but we cut off at 100MB
// as we will always know the size for everything but a timestamp and
// sometimes a root, neither of which should be exceptionally large
func (m *MemoryStore) GetMeta(name string, size int64) ([]byte, error) {
	d, ok := m.meta[name]
	if ok {
		if size == -1 {
			size = notary.MaxDownloadSize
		}
		if int64(len(d)) < size {
			return d, nil
		}
		return d[:size], nil
	}
	d, ok = m.consistent[name]
	if ok {
		if int64(len(d)) < size {
			return d, nil
		}
		return d[:size], nil
	}
	return nil, ErrMetaNotFound{Resource: name}
}

// SetMeta sets the metadata value for the given name
func (m *MemoryStore) SetMeta(name string, meta []byte) error {
	m.meta[name] = meta

	checksum := sha256.Sum256(meta)
	path := utils.ConsistentName(name, checksum[:])
	m.consistent[path] = meta
	return nil
}

// SetMultiMeta sets multiple pieces of metadata for multiple names
// in a single operation.
func (m *MemoryStore) SetMultiMeta(metas map[string][]byte) error {
	for role, blob := range metas {
		m.SetMeta(role, blob)
	}
	return nil
}

// RemoveMeta removes the metadata for a single role - if the metadata doesn't
// exist, no error is returned
func (m *MemoryStore) RemoveMeta(name string) error {
	if meta, ok := m.meta[name]; ok {
		checksum := sha256.Sum256(meta)
		path := utils.ConsistentName(name, checksum[:])
		delete(m.meta, name)
		delete(m.consistent, path)
	}
	return nil
}

// GetKey returns the public key for the given role
func (m *MemoryStore) GetKey(role string) ([]byte, error) {
	return nil, fmt.Errorf("GetKey is not implemented for the MemoryStore")
}

// RemoveAll clears the existing memory store by setting this store as new empty one
func (m *MemoryStore) RemoveAll() error {
	*m = *NewMemoryStore(nil)
	return nil
}
