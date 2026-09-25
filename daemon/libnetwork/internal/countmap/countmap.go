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
