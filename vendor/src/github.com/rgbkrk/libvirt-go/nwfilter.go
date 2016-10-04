package libvirt

/*
#cgo LDFLAGS: -lvirt 
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"
)

type VirNWFilter struct {
	ptr C.virNWFilterPtr
}

func (f *VirNWFilter) Free() error {
	if result := C.virNWFilterFree(f.ptr); result != 0 {
		return GetLastError()
	}
	return nil
}

func (f *VirNWFilter) GetName() (string, error) {
	name := C.virNWFilterGetName(f.ptr)
	if name == nil {
		return "", GetLastError()
	}
	return C.GoString(name), nil
}

func (f *VirNWFilter) Undefine() error {
	result := C.virNWFilterUndefine(f.ptr)
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (f *VirNWFilter) GetUUID() ([]byte, error) {
	var cUuid [C.VIR_UUID_BUFLEN](byte)
	cuidPtr := unsafe.Pointer(&cUuid)
	result := C.virNWFilterGetUUID(f.ptr, (*C.uchar)(cuidPtr))
	if result != 0 {
		return []byte{}, GetLastError()
	}
	return C.GoBytes(cuidPtr, C.VIR_UUID_BUFLEN), nil
}

func (f *VirNWFilter) GetUUIDString() (string, error) {
	var cUuid [C.VIR_UUID_STRING_BUFLEN](C.char)
	cuidPtr := unsafe.Pointer(&cUuid)
	result := C.virNWFilterGetUUIDString(f.ptr, (*C.char)(cuidPtr))
	if result != 0 {
		return "", GetLastError()
	}
	return C.GoString((*C.char)(cuidPtr)), nil
}

func (f *VirNWFilter) GetXMLDesc(flags uint32) (string, error) {
	result := C.virNWFilterGetXMLDesc(f.ptr, C.uint(flags))
	if result == nil {
		return "", GetLastError()
	}
	xml := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return xml, nil
}
