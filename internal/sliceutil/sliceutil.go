// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

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

func Map[S ~[]In, In, Out any](s S, fn func(In) Out) []Out {
	res := make([]Out, len(s))
	for i, v := range s {
		res[i] = fn(v)
	}
	return res
}

func Mapper[In, Out any](fn func(In) Out) func([]In) []Out {
	return func(s []In) []Out {
		res := make([]Out, len(s))
		for i, v := range s {
			res[i] = fn(v)
		}
		return res
	}
}
