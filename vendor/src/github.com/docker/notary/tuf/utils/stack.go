package utils

import (
	"fmt"
	"sync"
)

// ErrEmptyStack is used when an action that requires some
// content is invoked and the stack is empty
type ErrEmptyStack struct {
	action string
}

func (err ErrEmptyStack) Error() string {
	return fmt.Sprintf("attempted to %s with empty stack", err.action)
}

// ErrBadTypeCast is used by PopX functions when the item
// cannot be typed to X
type ErrBadTypeCast struct{}

func (err ErrBadTypeCast) Error() string {
	return "attempted to do a typed pop and item was not of type"
}

// Stack is a simple type agnostic stack implementation
type Stack struct {
	s []interface{}
	l sync.Mutex
}

// NewStack create a new stack
func NewStack() *Stack {
	s := &Stack{
		s: make([]interface{}, 0),
	}
	return s
}

// Push adds an item to the top of the stack.
func (s *Stack) Push(item interface{}) {
	s.l.Lock()
	defer s.l.Unlock()
	s.s = append(s.s, item)
}

// Pop removes and returns the top item on the stack, or returns
// ErrEmptyStack if the stack has no content
func (s *Stack) Pop() (interface{}, error) {
	s.l.Lock()
	defer s.l.Unlock()
	l := len(s.s)
	if l > 0 {
		item := s.s[l-1]
		s.s = s.s[:l-1]
		return item, nil
	}
	return nil, ErrEmptyStack{action: "Pop"}
}

// PopString attempts to cast the top item on the stack to the string type.
// If this succeeds, it removes and returns the top item. If the item
// is not of the string type, ErrBadTypeCast is returned. If the stack
// is empty, ErrEmptyStack is returned
func (s *Stack) PopString() (string, error) {
	s.l.Lock()
	defer s.l.Unlock()
	l := len(s.s)
	if l > 0 {
		item := s.s[l-1]
		if item, ok := item.(string); ok {
			s.s = s.s[:l-1]
			return item, nil
		}
		return "", ErrBadTypeCast{}
	}
	return "", ErrEmptyStack{action: "PopString"}
}

// Empty returns true if the stack is empty
func (s *Stack) Empty() bool {
	s.l.Lock()
	defer s.l.Unlock()
	return len(s.s) == 0
}
