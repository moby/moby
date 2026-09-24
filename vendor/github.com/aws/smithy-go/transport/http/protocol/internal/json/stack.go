package json

// chosen as an arbitrary average max depth for the RPC-style payloads we
// typically deal with in an SDK client
const initialCap = 8

type stackT[T any] struct {
	values   []T
	sentinel T
}

func newStackT[T any](sentinel T) stackT[T] {
	return stackT[T]{
		values:   make([]T, 0, initialCap),
		sentinel: sentinel,
	}
}

func (s *stackT[T]) Push(v T) {
	s.values = append(s.values, v)
}

func (s *stackT[T]) Pop() T {
	if len(s.values) == 0 {
		return s.sentinel
	}
	idx := len(s.values) - 1
	v := s.values[idx]
	s.values = s.values[:idx]
	return v
}

func (s *stackT[T]) Top() T {
	if len(s.values) == 0 {
		return s.sentinel
	}
	return s.values[len(s.values)-1]
}
