package bkmaps

import "sync"

// SyncMap provides a typed wrapper around sync.Map.
type SyncMap[K comparable, V any] struct {
	m sync.Map
}

// Delete removes the value for a key.
func (m *SyncMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

// Load returns the value stored in the map for a key, if any.
func (m *SyncMap[K, V]) Load(key K) (V, bool) {
	v, ok := m.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise it stores and returns the given value.
func (m *SyncMap[K, V]) LoadOrStore(key K, value V) (V, bool) {
	v, loaded := m.m.LoadOrStore(key, value)
	return v.(V), loaded
}

// Range calls fn sequentially for each key and value present in the map.
func (m *SyncMap[K, V]) Range(fn func(K, V) bool) {
	m.m.Range(func(key, value any) bool {
		return fn(key.(K), value.(V))
	})
}

// Store sets the value for a key.
func (m *SyncMap[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}
