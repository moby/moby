package platform

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func munmapCodeSegment(code []byte) error {
	address := unsafe.Pointer(&code[0])
	size := uintptr(0) // size must be 0 because we're using MEM_RELEASE.
	return windows.VirtualFree(uintptr(address), size, windows.MEM_RELEASE)
}

func mmapCodeSegment(size int) ([]byte, error) {
	address := uintptr(0) // system determines where to allocate the region.
	p, err := windows.VirtualAlloc(address, uintptr(size),
		windows.MEM_COMMIT, windows.PAGE_READWRITE)
	if err != nil {
		return nil, err
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(p)), size), nil
}

var old = uint32(windows.PAGE_READWRITE)

func MprotectCodeSegment(b []byte) (err error) {
	address := unsafe.Pointer(&b[0])
	return windows.VirtualProtect(uintptr(address), uintptr(len(b)), windows.PAGE_EXECUTE_READ, &old)
}
