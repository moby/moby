// Package wasmruntime contains internal symbols shared between modules for error handling.
// Note: This is named wasmruntime to avoid conflicts with the normal go module.
// Note: This only imports "api" as importing "wasm" would create a cyclic dependency.
package wasmruntime

var (
	// ErrRuntimeStackOverflow indicates that there are too many function calls,
	// and the Engine terminated the execution.
	ErrRuntimeStackOverflow = New("stack overflow")
	// ErrRuntimeInvalidConversionToInteger indicates the Wasm function tries to
	// convert NaN floating point value to integers during trunc variant instructions.
	ErrRuntimeInvalidConversionToInteger = New("invalid conversion to integer")
	// ErrRuntimeIntegerOverflow indicates that an integer arithmetic resulted in
	// overflow value. For example, when the program tried to truncate a float value
	// which doesn't fit in the range of target integer.
	ErrRuntimeIntegerOverflow = New("integer overflow")
	// ErrRuntimeIntegerDivideByZero indicates that an integer div or rem instructions
	// was executed with 0 as the divisor.
	ErrRuntimeIntegerDivideByZero = New("integer divide by zero")
	// ErrRuntimeUnreachable means "unreachable" instruction was executed by the program.
	ErrRuntimeUnreachable = New("unreachable")
	// ErrRuntimeOutOfBoundsMemoryAccess indicates that the program tried to access the
	// region beyond the linear memory.
	ErrRuntimeOutOfBoundsMemoryAccess = New("out of bounds memory access")
	// ErrRuntimeInvalidTableAccess means either offset to the table was out of bounds of table, or
	// the target element in the table was uninitialized during call_indirect instruction.
	ErrRuntimeInvalidTableAccess = New("invalid table access")
	// ErrRuntimeIndirectCallTypeMismatch indicates that the type check failed during call_indirect.
	ErrRuntimeIndirectCallTypeMismatch = New("indirect call type mismatch")
	// ErrRuntimeUnalignedAtomic indicates that an atomic operation was made with incorrect memory alignment.
	ErrRuntimeUnalignedAtomic = New("unaligned atomic")
	// ErrRuntimeExpectedSharedMemory indicates that an operation was made against unshared memory when not allowed.
	ErrRuntimeExpectedSharedMemory = New("expected shared memory")
	// ErrRuntimeTooManyWaiters indicates that atomic.wait was called with too many waiters.
	ErrRuntimeTooManyWaiters = New("too many waiters")
)

// Error is returned by a wasm.Engine during the execution of Wasm functions, and they indicate that the Wasm runtime
// state is unrecoverable.
type Error struct {
	s string
}

func New(text string) *Error {
	return &Error{s: text}
}

func (e *Error) Error() string {
	return e.s
}
