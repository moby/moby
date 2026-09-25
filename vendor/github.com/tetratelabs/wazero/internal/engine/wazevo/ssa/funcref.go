package ssa

import "fmt"

// FuncRef is a unique identifier for a function of the frontend,
// and is used to reference the function in function call.
type FuncRef uint32

// String implements fmt.Stringer.
func (r FuncRef) String() string {
	return fmt.Sprintf("f%d", r)
}
