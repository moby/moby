//go:build wasip1 && !tinygo.wasm

// This file is designed to be imported by plugins.

package wasm

import (
	"unsafe"
)

// allocations keeps track of each allocated byte slice, keyed by a fake pointer.
// This map ensures the GC will not collect these slices while still in use.
var allocations = make(map[uint32][]byte)

// allocate creates a new byte slice of the given size and stores it in the
// allocations map so that it remains valid (not garbage-collected).
func allocate(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	// Create a new byte slice on the Go heap
	b := make([]byte, size)

	// Obtain the 'address' of the first element in b by converting its pointer to a uint32.
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))

	// Store the byte slice in the map, keyed by the pointer
	allocations[ptr] = b
	return ptr
}

//go:wasmexport malloc
func Malloc(size uint32) uint32 {
	return allocate(size)
}

//go:wasmexport free
func Free(ptr uint32) {
	// Remove the slice from the allocations map so the GC can reclaim it later.
	delete(allocations, ptr)
}

func PtrToByte(ptr, size uint32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)
}

func ByteToPtr(buf []byte) (uint32, uint32) {
	if len(buf) == 0 {
		return 0, 0
	}
	ptr := &buf[0]
	unsafePtr := uintptr(unsafe.Pointer(ptr))
	return uint32(unsafePtr), uint32(len(buf))
}
