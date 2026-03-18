package maputil

func FilterValues[K comparable, V any](in map[K]V, fn func(V) bool) []V {
	var out []V
	for _, v := range in {
		if fn(v) {
			out = append(out, v)
		}
	}
	return out
}
