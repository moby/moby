// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

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
