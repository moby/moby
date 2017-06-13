package common

import (
	"sync"

	mapset "github.com/deckarep/golang-set"
)

// SetMatrix is a map of Sets
type SetMatrix interface {
	// Get returns the members of the set for a specific key as a slice.
	Get(key string) ([]interface{}, bool)
	// Contains is used to verify is an element is in a set for a specific key
	// returns true if the element is in the set
	// returns true if there is a set for the key
	Contains(key string, value interface{}) (bool, bool)
	// Insert inserts the mapping between the IP and the endpoint identifier
	// returns true if the mapping was not present, false otherwise
	// returns also the number of endpoints associated to the IP
	Insert(key string, value interface{}) (bool, int)
	// Remove removes the mapping between the IP and the endpoint identifier
	// returns true if the mapping was deleted, false otherwise
	// returns also the number of endpoints associated to the IP
	Remove(key string, value interface{}) (bool, int)
	// Cardinality returns the number of elements in the set of a specfic key
	// returns false if the key is not in the map
	Cardinality(key string) (int, bool)
	// String returns the string version of the set, empty otherwise
	// returns false if the key is not in the map
	String(key string) (string, bool)
}

type setMatrix struct {
	matrix map[string]mapset.Set

	sync.Mutex
}

// NewSetMatrix creates a new set matrix object
func NewSetMatrix() SetMatrix {
	s := &setMatrix{}
	s.init()
	return s
}

func (s *setMatrix) init() {
	s.matrix = make(map[string]mapset.Set)
}

func (s *setMatrix) Get(key string) ([]interface{}, bool) {
	s.Lock()
	defer s.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return nil, ok
	}
	return set.ToSlice(), ok
}

func (s *setMatrix) Contains(key string, value interface{}) (bool, bool) {
	s.Lock()
	defer s.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return false, ok
	}
	return set.Contains(value), ok
}

func (s *setMatrix) Insert(key string, value interface{}) (bool, int) {
	s.Lock()
	defer s.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		s.matrix[key] = mapset.NewSet()
		s.matrix[key].Add(value)
		return true, 1
	}

	return set.Add(value), set.Cardinality()
}

func (s *setMatrix) Remove(key string, value interface{}) (bool, int) {
	s.Lock()
	defer s.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return false, 0
	}

	var removed bool
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

func (s *setMatrix) Cardinality(key string) (int, bool) {
	s.Lock()
	defer s.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return 0, ok
	}

	return set.Cardinality(), ok
}

func (s *setMatrix) String(key string) (string, bool) {
	s.Lock()
	defer s.Unlock()
	set, ok := s.matrix[key]
	if !ok {
		return "", ok
	}
	return set.String(), ok
}
