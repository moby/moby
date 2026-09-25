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

// Map applies a function to each element of the input sequence.
func Map[T, U any](s iter.Seq[T], fn func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range s {
			if !yield(fn(v)) {
				return
			}
		}
	}
}

// Map2 applies a function to each element of the input sequence.
func Map2[K1, V1, K2, V2 any](s iter.Seq2[K1, V1], fn func(K1, V1) (K2, V2)) iter.Seq2[K2, V2] {
	return func(yield func(K2, V2) bool) {
		for k1, v1 := range s {
			k2, v2 := fn(k1, v1)
			if !yield(k2, v2) {
				return
			}
		}
	}
}
