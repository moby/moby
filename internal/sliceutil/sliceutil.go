// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.19

package sliceutil

func Dedup[T comparable](slice []T) []T {
	keys := make(map[T]struct{})
	out := make([]T, 0, len(slice))
	for _, s := range slice {
		if _, ok := keys[s]; !ok {
			out = append(out, s)
			keys[s] = struct{}{}
		}
	}
	return out
}
