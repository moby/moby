package wazevoapi

import "unsafe"

// PtrFromUintptr resurrects the original *T from the given uintptr.
// The caller of this function MUST be sure that ptr is valid.
func PtrFromUintptr[T any](ptr uintptr) *T {
	// Wraps ptrs as the double pointer in order to avoid the unsafe access as detected by race detector.
	//
	// For example, if we have (*function)(unsafe.Pointer(ptr)) instead, then the race detector's "checkptr"
	// subroutine wanrs as "checkptr: pointer arithmetic result points to invalid allocation"
	// https://github.com/golang/go/blob/1ce7fcf139417d618c2730010ede2afb41664211/src/runtime/checkptr.go#L69
	var wrapped *uintptr = &ptr
	return *(**T)(unsafe.Pointer(wrapped))
}
