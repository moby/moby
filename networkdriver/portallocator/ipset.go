package ipallocator

import (
	"sync"
)

// iPSet is a thread-safe sorted set and a stack.
type iPSet struct {
	sync.RWMutex
	set []string
}

// Push takes a string and adds it to the set. If the elem aready exists, it has no effect.
func (s *iPSet) Push(elem string) {
	s.RLock()
	for i, e := range s.set {
		if e == elem {
			s.RUnlock()
			return
		}
	}
	s.RUnlock()

	s.Lock()
	s.set = append(s.set, elem)
	// Make sure the list is always sorted
	sort.Strings(s.set)
	s.Unlock()
}

// Pop returns the first elemen from the list and removes it.
// If the list is empty, it returns an empty string
func (s *iPSet) Pop() string {
	s.RLock()

	for i, e := range s.set {
		ret := e
		s.RUnlock()
		s.Lock()
		s.set = append(s.set[:i], s.set[i+1:]...)
		s.Unlock()
		return e
	}
	s.RUnlock()
	return ""
}

// Remove removes an element from the list.
// If the element is not found, it has no effect.
func (s *iPSet) Remove(elem string) {
	for i, e := range s.set {
		if e == elem {
			s.set = append(s.set[:i], s.set[i+1:]...)
			return
		}
	}
}

// Len returns the length of the list.
func (s *iPSet) Len() int {
	return len(s.set)
}
