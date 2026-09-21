package sys

import (
	"unsafe"

	"github.com/cilium/ebpf/internal/unix"
)

// UnsafePointer creates a 64-bit pointer from an unsafe Pointer.
func UnsafePointer(ptr unsafe.Pointer) Pointer {
	return Pointer{ptr: ptr}
}

// UnsafeSlicePointer creates an untyped [Pointer] from a slice.
func UnsafeSlicePointer[T comparable](buf []T) Pointer {
	if len(buf) == 0 {
		return Pointer{}
	}

	return Pointer{ptr: unsafe.Pointer(unsafe.SliceData(buf))}
}

// TypedPointer points to typed memory.
//
// It is like a *T except that it accounts for the BPF syscall interface.
type TypedPointer[T any] struct {
	_   [0]*T // prevent TypedPointer[a] to be convertible to TypedPointer[b]
	ptr Pointer
}

func (p TypedPointer[T]) IsNil() bool {
	return p.ptr.ptr == nil
}

// SlicePointer creates a [TypedPointer] from a slice.
func SlicePointer[T comparable](s []T) TypedPointer[T] {
	return TypedPointer[T]{ptr: UnsafeSlicePointer(s)}
}

// StringPointer points to a null-terminated string.
type StringPointer struct {
	_   [0]string
	ptr Pointer
}

// NewStringPointer creates a [StringPointer] from a string.
func NewStringPointer(str string) StringPointer {
	slice, err := unix.ByteSliceFromString(str)
	if err != nil {
		return StringPointer{}
	}

	return StringPointer{ptr: Pointer{ptr: unsafe.Pointer(&slice[0])}}
}

// StringSlicePointer points to a slice of [StringPointer].
type StringSlicePointer struct {
	_   [0][]string
	ptr Pointer
}

// NewStringSlicePointer allocates an array of Pointers to each string in the
// given slice of strings and returns a 64-bit pointer to the start of the
// resulting array.
//
// Use this function to pass arrays of strings as syscall arguments.
func NewStringSlicePointer(strings []string) StringSlicePointer {
	sp := make([]StringPointer, 0, len(strings))
	for _, s := range strings {
		sp = append(sp, NewStringPointer(s))
	}

	return StringSlicePointer{ptr: Pointer{ptr: unsafe.Pointer(&sp[0])}}
}
