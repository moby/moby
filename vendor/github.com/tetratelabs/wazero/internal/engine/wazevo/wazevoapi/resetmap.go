package wazevoapi

// ResetMap resets the map to an empty state, or creates a new map if it is nil.
func ResetMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		m = make(map[K]V)
	} else {
		clear(m)
	}
	return m
}
