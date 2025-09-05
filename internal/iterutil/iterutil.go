package iterutil

import (
	"iter"
	"maps"
)

// SameValues checks if a and b yield the same values, independent of order.
func SameValues[T comparable](a, b iter.Seq[T]) bool {
	m, n := make(map[T]int), make(map[T]int)
	for v := range a {
		m[v]++
	}
	for v := range b {
		n[v]++
	}
	return maps.Equal(m, n)
}

// Deref adapts an iterator of pointers to an iterator of values.
func Deref[T any, P *T](s iter.Seq[P]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for p := range s {
			if !yield(*p) {
				return
			}
		}
	}
}

// Chain concatenates multiple iterators into a single iterator.
func Chain[T any](iters ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, it := range iters {
			for v := range it {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Chain2 concatenates multiple iterators into a single iterator.
func Chain2[K, V any](iters ...iter.Seq2[K, V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, it := range iters {
			for k, v := range it {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}
