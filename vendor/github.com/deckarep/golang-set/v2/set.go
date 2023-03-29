/*
Open Source Initiative OSI - The MIT License (MIT):Licensing

The MIT License (MIT)
Copyright (c) 2013 - 2022 Ralph Caraveo (deckarep@gmail.com)

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package mapset implements a simple and  set collection.
// Items stored within it are unordered and unique. It supports
// typical set operations: membership testing, intersection, union,
// difference, symmetric difference and cloning.
//
// Package mapset provides two implementations of the Set
// interface. The default implementation is safe for concurrent
// access, but a non-thread-safe implementation is also provided for
// programs that can benefit from the slight speed improvement and
// that can enforce mutual exclusion through other means.
package mapset

// Set is the primary interface provided by the mapset package.  It
// represents an unordered set of data and a large number of
// operations that can be applied to that set.
type Set[T comparable] interface {
	// Add adds an element to the set. Returns whether
	// the item was added.
	Add(val T) bool

	// Append multiple elements to the set. Returns
	// the number of elements added.
	Append(val ...T) int

	// Cardinality returns the number of elements in the set.
	Cardinality() int

	// Clear removes all elements from the set, leaving
	// the empty set.
	Clear()

	// Clone returns a clone of the set using the same
	// implementation, duplicating all keys.
	Clone() Set[T]

	// Contains returns whether the given items
	// are all in the set.
	Contains(val ...T) bool

	// Difference returns the difference between this set
	// and other. The returned set will contain
	// all elements of this set that are not also
	// elements of other.
	//
	// Note that the argument to Difference
	// must be of the same type as the receiver
	// of the method. Otherwise, Difference will
	// panic.
	Difference(other Set[T]) Set[T]

	// Equal determines if two sets are equal to each
	// other. If they have the same cardinality
	// and contain the same elements, they are
	// considered equal. The order in which
	// the elements were added is irrelevant.
	//
	// Note that the argument to Equal must be
	// of the same type as the receiver of the
	// method. Otherwise, Equal will panic.
	Equal(other Set[T]) bool

	// Intersect returns a new set containing only the elements
	// that exist only in both sets.
	//
	// Note that the argument to Intersect
	// must be of the same type as the receiver
	// of the method. Otherwise, Intersect will
	// panic.
	Intersect(other Set[T]) Set[T]

	// IsProperSubset determines if every element in this set is in
	// the other set but the two sets are not equal.
	//
	// Note that the argument to IsProperSubset
	// must be of the same type as the receiver
	// of the method. Otherwise, IsProperSubset
	// will panic.
	IsProperSubset(other Set[T]) bool

	// IsProperSuperset determines if every element in the other set
	// is in this set but the two sets are not
	// equal.
	//
	// Note that the argument to IsSuperset
	// must be of the same type as the receiver
	// of the method. Otherwise, IsSuperset will
	// panic.
	IsProperSuperset(other Set[T]) bool

	// IsSubset determines if every element in this set is in
	// the other set.
	//
	// Note that the argument to IsSubset
	// must be of the same type as the receiver
	// of the method. Otherwise, IsSubset will
	// panic.
	IsSubset(other Set[T]) bool

	// IsSuperset determines if every element in the other set
	// is in this set.
	//
	// Note that the argument to IsSuperset
	// must be of the same type as the receiver
	// of the method. Otherwise, IsSuperset will
	// panic.
	IsSuperset(other Set[T]) bool

	// Each iterates over elements and executes the passed func against each element.
	// If passed func returns true, stop iteration at the time.
	Each(func(T) bool)

	// Iter returns a channel of elements that you can
	// range over.
	Iter() <-chan T

	// Iterator returns an Iterator object that you can
	// use to range over the set.
	Iterator() *Iterator[T]

	// Remove removes a single element from the set.
	Remove(i T)

	// RemoveAll removes multiple elements from the set.
	RemoveAll(i ...T)

	// String provides a convenient string representation
	// of the current state of the set.
	String() string

	// SymmetricDifference returns a new set with all elements which are
	// in either this set or the other set but not in both.
	//
	// Note that the argument to SymmetricDifference
	// must be of the same type as the receiver
	// of the method. Otherwise, SymmetricDifference
	// will panic.
	SymmetricDifference(other Set[T]) Set[T]

	// Union returns a new set with all elements in both sets.
	//
	// Note that the argument to Union must be of the
	// same type as the receiver of the method.
	// Otherwise, IsSuperset will panic.
	Union(other Set[T]) Set[T]

	// Pop removes and returns an arbitrary item from the set.
	Pop() (T, bool)

	// ToSlice returns the members of the set as a slice.
	ToSlice() []T

	// MarshalJSON will marshal the set into a JSON-based representation.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON will unmarshal a JSON-based byte slice into a full Set datastructure.
	// For this to work, set subtypes must implemented the Marshal/Unmarshal interface.
	UnmarshalJSON(b []byte) error
}

// NewSet creates and returns a new set with the given elements.
// Operations on the resulting set are thread-safe.
func NewSet[T comparable](vals ...T) Set[T] {
	s := newThreadSafeSetWithSize[T](len(vals))
	for _, item := range vals {
		s.Add(item)
	}
	return s
}

// NewSetWithSize creates and returns a reference to an empty set with a specified
// capacity. Operations on the resulting set are thread-safe.
func NewSetWithSize[T comparable](cardinality int) Set[T] {
	s := newThreadSafeSetWithSize[T](cardinality)
	return s
}

// NewThreadUnsafeSet creates and returns a new set with the given elements.
// Operations on the resulting set are not thread-safe.
func NewThreadUnsafeSet[T comparable](vals ...T) Set[T] {
	s := newThreadUnsafeSetWithSize[T](len(vals))
	for _, item := range vals {
		s.Add(item)
	}
	return s
}

// NewThreadUnsafeSetWithSize creates and returns a reference to an empty set with
// a specified capacity. Operations on the resulting set are not thread-safe.
func NewThreadUnsafeSetWithSize[T comparable](cardinality int) Set[T] {
	s := newThreadUnsafeSetWithSize[T](cardinality)
	return s
}

// NewSetFromMapKeys creates and returns a new set with the given keys of the map.
// Operations on the resulting set are thread-safe.
func NewSetFromMapKeys[T comparable, V any](val map[T]V) Set[T] {
	s := NewSetWithSize[T](len(val))

	for k := range val {
		s.Add(k)
	}

	return s
}

// NewThreadUnsafeSetFromMapKeys creates and returns a new set with the given keys of the map.
// Operations on the resulting set are not thread-safe.
func NewThreadUnsafeSetFromMapKeys[T comparable, V any](val map[T]V) Set[T] {
	s := NewThreadUnsafeSetWithSize[T](len(val))

	for k := range val {
		s.Add(k)
	}

	return s
}
