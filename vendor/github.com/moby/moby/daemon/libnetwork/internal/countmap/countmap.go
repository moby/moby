// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package countmap

// Map is a map of counters.
type Map[T comparable] map[T]int

// Add adds delta to the counter for v and returns the new value.
//
// If the new value is 0, the entry is removed from the map.
func (m Map[T]) Add(v T, delta int) int {
	m[v] += delta
	c := m[v]
	if c == 0 {
		delete(m, v)
	}
	return c
}
