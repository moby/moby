package setmatrix

import (
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
)

// SetMatrix is a map of Sets.
// The zero value is an empty set matrix ready to use.
//
// SetMatrix values are safe for concurrent use.
type SetMatrix[T comparable] struct {
	matrix map[string]mapset.Set[T]

	mu sync.Mutex
}

// Get returns the members of the set for a specific key as a slice.
func (s *SetMatrix[T]) Get(key string) ([]T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return nil, ok
	}
	return set.ToSlice(), ok
}

// Contains is used to verify if an element is in a set for a specific key.
func (s *SetMatrix[T]) Contains(key string, value T) (containsElement, setExists bool) {
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
func (s *SetMatrix[T]) Insert(key string, value T) (inserted bool, cardinality int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		if s.matrix == nil {
			s.matrix = make(map[string]mapset.Set[T])
		}
		s.matrix[key] = mapset.NewThreadUnsafeSet(value)
		return true, 1
	}

	return set.Add(value), set.Cardinality()
}

// Remove removes the value in the set for a specific key.
func (s *SetMatrix[T]) Remove(key string, value T) (removed bool, cardinality int) {
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
func (s *SetMatrix[T]) Cardinality(key string) (cardinality int, ok bool) {
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
func (s *SetMatrix[T]) String(key string) (v string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return "", ok
	}
	return set.String(), ok
}

// Keys returns all the keys in the map.
func (s *SetMatrix[T]) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.matrix))
	for k := range s.matrix {
		keys = append(keys, k)
	}
	return keys
}
