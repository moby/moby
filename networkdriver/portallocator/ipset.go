package ipallocator

import (
	"sort"
	"sync"
)

// iPSet is a thread-safe sorted set and a stack.
type iPSet struct {
	sync.RWMutex
	set []int32
}

// Push takes a string and adds it to the set. If the elem aready exists, it has no effect.
func (s *iPSet) Push(elem int32) {
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
	sort.Ints(s.set)
	s.Unlock()
}

// Pop is an alias to PopFront()
func (s *iPSet) Pop() int32 {
	return s.PopFront()
}

// Pop returns the first elemen from the list and removes it.
// If the list is empty, it returns 0
func (s *iPSet) PopFront() int32 {
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

// PullBack retrieve the last element of the list.
// The element is not removed.
// If the list is empty, an empty element is returned.
func (s *iPSet) PullBack() int32 {
	if len(s.set) == 0 {
		return ""
	}
	return s.set[len(s.set)-1]
}

// Exists checks if the given element present in the list.
func (s *iPSet) Exists(elem int32) bool {
	for _, e := range s.set {
		if e == elem {
			return true
		}
	}
	return false
}

// Remove removes an element from the list.
// If the element is not found, it has no effect.
func (s *iPSet) Remove(elem int32) {
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
