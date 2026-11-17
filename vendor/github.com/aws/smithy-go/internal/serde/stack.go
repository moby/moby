package serde

// Stack is a generic slice-backed stack pre-allocated to avoid growth
// allocations for typical serde nesting depths.
type Stack[T any] struct {
	values []T
}

// NewStack returns a Stack pre-allocated with capacity 8.
func NewStack[T any]() Stack[T] {
	return Stack[T]{values: make([]T, 0, 8)}
}

// Push adds a value to the top of the stack.
func (s *Stack[T]) Push(v T) {
	s.values = append(s.values, v)
}

// Pop removes and returns the top value.
func (s *Stack[T]) Pop() T {
	v := s.values[len(s.values)-1]
	var zero T
	s.values[len(s.values)-1] = zero
	s.values = s.values[:len(s.values)-1]
	return v
}

// Top returns a pointer to the top value without removing it.
func (s *Stack[T]) Top() *T {
	if len(s.values) == 0 {
		return nil
	}
	return &s.values[len(s.values)-1]
}

// Len returns the number of elements in the stack.
func (s *Stack[T]) Len() int {
	return len(s.values)
}

// Reset clears the stack while retaining allocated capacity.
func (s *Stack[T]) Reset() {
	var zero T
	for i := range s.values {
		s.values[i] = zero
	}
	s.values = s.values[:0]
}

// Values returns the underlying slice for indexed access.
func (s *Stack[T]) Values() []T {
	return s.values
}
