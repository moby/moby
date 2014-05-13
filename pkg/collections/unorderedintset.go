package collections

import (
	"sync"
)

// UnorderedIntSet is a thread-safe set and a queue.
type UnorderedIntSet struct {
	sync.RWMutex
	set []int
}

// NewOrderedSet returns an initialized OrderedSet
func NewUnorderedIntSet() *UnorderedIntSet {
	return &UnorderedIntSet{}
}

// Push is an alias for PushBack(int)
func (s *UnorderedIntSet) Push(elem int) {
	s.PushBack(elem)
}

// Push takes a string and adds it to the set. If the elem aready exists, it has no effect.
func (s *UnorderedIntSet) PushBack(elem int) {
	s.RLock()
	for _, e := range s.set {
		if e == elem {
			s.RUnlock()
			return
		}
	}
	s.RUnlock()

	s.Lock()
	// simply append to the end of queue
	s.set = append(s.set, elem)
	s.Unlock()
}

// Pop is an alias to PopFront()
func (s *UnorderedIntSet) Pop() int {
	return s.PopFront()
}

// Pop returns the first elemen from the list and removes it.
// If the list is empty, it returns 0
func (s *UnorderedIntSet) PopFront() int {
	s.RLock()

	for i, e := range s.set {
		ret := e
		s.RUnlock()
		s.Lock()
		s.set = append(s.set[:i], s.set[i+1:]...)
		s.Unlock()
		return ret
	}
	s.RUnlock()

	return 0
}

// Exists checks if the given element present in the list.
func (s *UnorderedIntSet) Exists(elem int) bool {
	for _, e := range s.set {
		if e == elem {
			return true
		}
	}
	return false
}

// Remove removes an element from the list.
// If the element is not found, it has no effect.
func (s *UnorderedIntSet) Remove(elem int) {
	for i, e := range s.set {
		if e == elem {
			s.set = append(s.set[:i], s.set[i+1:]...)
			return
		}
	}
}
