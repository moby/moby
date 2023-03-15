package setmatrix

import (
	"sync"

	mapset "github.com/deckarep/golang-set"
)

// SetMatrix is a map of Sets.
type SetMatrix struct {
	matrix map[string]mapset.Set

	mu sync.Mutex
}

// NewSetMatrix creates a new set matrix object.
func NewSetMatrix() *SetMatrix {
	s := &SetMatrix{}
	s.init()
	return s
}

func (s *SetMatrix) init() {
	s.matrix = make(map[string]mapset.Set)
}

// Get returns the members of the set for a specific key as a slice.
func (s *SetMatrix) Get(key string) ([]interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return nil, ok
	}
	return set.ToSlice(), ok
}

// Contains is used to verify if an element is in a set for a specific key.
func (s *SetMatrix) Contains(key string, value interface{}) (containsElement, setExists bool) {
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
func (s *SetMatrix) Insert(key string, value interface{}) (insetrted bool, cardinality int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		s.matrix[key] = mapset.NewSet()
		s.matrix[key].Add(value)
		return true, 1
	}

	return set.Add(value), set.Cardinality()
}

// Remove removes the value in the set for a specific key.
func (s *SetMatrix) Remove(key string, value interface{}) (removed bool, cardinality int) {
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
func (s *SetMatrix) Cardinality(key string) (cardinality int, ok bool) {
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
func (s *SetMatrix) String(key string) (v string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return "", ok
	}
	return set.String(), ok
}

// Keys returns all the keys in the map.
func (s *SetMatrix) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.matrix))
	for k := range s.matrix {
		keys = append(keys, k)
	}
	return keys
}
