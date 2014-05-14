package collections

import (
	"sort"
	"sync"
)

// OrderedIntSet is a thread-safe sorted set and a stack.
type OrderedIntSet struct {
	sync.Mutex
	set []int
}

// NewOrderedSet returns an initialized OrderedSet
func NewOrderedIntSet() *OrderedIntSet {
	return &OrderedIntSet{}
}

// Push takes an int and adds it to the set. If the elem aready exists, it has no effect.
func (s *OrderedIntSet) Push(elem int) {
	s.Lock()
	if len(s.set) == 0 {
		s.set = append(s.set, elem)
		s.Unlock()
		return
	}

	// Make sure the list is always sorted
	i := sort.SearchInts(s.set, elem)
	if i < len(s.set) && s.set[i] == elem {
		s.Unlock()
		return
	}
	s.set = append(s.set[:i], append([]int{elem}, s.set[i:]...)...)
	s.Unlock()
}

// Pop is an alias to PopFront()
func (s *OrderedIntSet) Pop() int {
	return s.PopFront()
}

// Pop returns the first element from the list and removes it.
// If the list is empty, it returns 0
func (s *OrderedIntSet) PopFront() int {
	s.Lock()
	if len(s.set) == 0 {
		s.Unlock()
		return 0
	}
	ret := s.set[0]
	s.set = s.set[1:]
	s.Unlock()
	return ret
}

// PullBack retrieve the last element of the list.
// The element is not removed.
// If the list is empty, an empty element is returned.
func (s *OrderedIntSet) PullBack() int {
	s.Lock()
	defer s.Unlock()
	if len(s.set) == 0 {
		return 0
	}
	return s.set[len(s.set)-1]
}

// Exists checks if the given element present in the list.
func (s *OrderedIntSet) Exists(elem int) bool {
	s.Lock()
	if len(s.set) == 0 {
		s.Unlock()
		return false
	}
	i := sort.SearchInts(s.set, elem)
	res := i < len(s.set) && s.set[i] == elem
	s.Unlock()
	return res
}

// Remove removes an element from the list.
// If the element is not found, it has no effect.
func (s *OrderedIntSet) Remove(elem int) {
	s.Lock()
	if len(s.set) == 0 {
		s.Unlock()
		return
	}
	i := sort.SearchInts(s.set, elem)
	if i < len(s.set) && s.set[i] == elem {
		s.set = append(s.set[:i], s.set[i+1:]...)
	}
	s.Unlock()
}
