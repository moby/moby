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
