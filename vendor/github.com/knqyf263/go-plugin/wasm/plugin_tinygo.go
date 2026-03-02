//go:build tinygo.wasm

// This file is designed to be imported by plugins.

package wasm

// #include <stdlib.h>
import "C"

import (
	"unsafe"
)

func PtrToByte(ptr, size uint32) []byte {
	b := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)

	return b
}

func ByteToPtr(buf []byte) (uint32, uint32) {
	if len(buf) == 0 {
		return 0, 0
	}

	size := C.ulong(len(buf))
	ptr := unsafe.Pointer(C.malloc(size))

	copy(unsafe.Slice((*byte)(ptr), size), buf)

	return uint32(uintptr(ptr)), uint32(len(buf))
}

func Free(ptr uint32) {
	C.free(unsafe.Pointer(uintptr(ptr)))
}
