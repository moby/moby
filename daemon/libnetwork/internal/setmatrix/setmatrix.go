package setmatrix

import (
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
)

// SetMatrix is a map of Sets.
// The zero value is an empty set matrix ready to use.
//
// SetMatrix values are safe for concurrent use.
type SetMatrix[K, V comparable] struct {
	matrix map[K]mapset.Set[V]

	mu sync.Mutex
}

// Get returns the members of the set for a specific key as a slice.
func (s *SetMatrix[K, V]) Get(key K) ([]V, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return nil, ok
	}
	return set.ToSlice(), ok
}

// Contains is used to verify if an element is in a set for a specific key.
func (s *SetMatrix[K, V]) Contains(key K, value V) (containsElement, setExists bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return false, ok
	}
	return set.Contains(value), ok
}

// Insert inserts the value in the set of a key and returns whether the value is
// inserted (was not already in the set) and the number of elements in the set.
func (s *SetMatrix[K, V]) Insert(key K, value V) (inserted bool, cardinality int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		if s.matrix == nil {
			s.matrix = make(map[K]mapset.Set[V])
		}
		s.matrix[key] = mapset.NewThreadUnsafeSet(value)
		return true, 1
	}

	return set.Add(value), set.Cardinality()
}

// Remove removes the value in the set for a specific key.
func (s *SetMatrix[K, V]) Remove(key K, value V) (removed bool, cardinality int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return false, 0
	}

	if set.Contains(value) {
		set.Remove(value)
		removed = true
		// If the set is empty remove it from the matrix
		if set.Cardinality() == 0 {
			delete(s.matrix, key)
		}
	}

	return removed, set.Cardinality()
}

// Cardinality returns the number of elements in the set for a key.
func (s *SetMatrix[K, V]) Cardinality(key K) (cardinality int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return 0, ok
	}

	return set.Cardinality(), ok
}

// String returns the string version of the set.
// The empty string is returned if there is no set for key.
func (s *SetMatrix[K, V]) String(key K) (v string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return "", ok
	}
	return set.String(), ok
}

// Keys returns all the keys in the map.
func (s *SetMatrix[K, V]) Keys() []K {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]K, 0, len(s.matrix))
	for k := range s.matrix {
		keys = append(keys, k)
	}
	return keys
}
