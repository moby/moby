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

type VirNetwork struct {
	ptr C.virNetworkPtr
}

func (n *VirNetwork) Free() error {
	if result := C.virNetworkFree(n.ptr); result != 0 {
		return GetLastError()
	}
	return nil
}

func (n *VirNetwork) Create() error {
	result := C.virNetworkCreate(n.ptr)
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (n *VirNetwork) Destroy() error {
	result := C.virNetworkDestroy(n.ptr)
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (n *VirNetwork) IsActive() (bool, error) {
	result := C.virNetworkIsActive(n.ptr)
	if result == -1 {
		return false, GetLastError()
	}
	if result == 1 {
		return true, nil
	}
	return false, nil
}

func (n *VirNetwork) IsPersistent() (bool, error) {
	result := C.virNetworkIsPersistent(n.ptr)
	if result == -1 {
		return false, GetLastError()
	}
	if result == 1 {
		return true, nil
	}
	return false, nil
}

func (n *VirNetwork) GetAutostart() (bool, error) {
	var out C.int
	result := C.virNetworkGetAutostart(n.ptr, (*C.int)(unsafe.Pointer(&out)))
	if result == -1 {
		return false, GetLastError()
	}
	switch out {
	case 1:
		return true, nil
	default:
		return false, nil
	}
}

func (n *VirNetwork) SetAutostart(autostart bool) error {
	var cAutostart C.int
	switch autostart {
	case true:
		cAutostart = 1
	default:
		cAutostart = 0
	}
	result := C.virNetworkSetAutostart(n.ptr, cAutostart)
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (n *VirNetwork) GetName() (string, error) {
	name := C.virNetworkGetName(n.ptr)
	if name == nil {
		return "", GetLastError()
	}
	return C.GoString(name), nil
}

func (n *VirNetwork) GetUUID() ([]byte, error) {
	var cUuid [C.VIR_UUID_BUFLEN](byte)
	cuidPtr := unsafe.Pointer(&cUuid)
	result := C.virNetworkGetUUID(n.ptr, (*C.uchar)(cuidPtr))
	if result != 0 {
		return []byte{}, GetLastError()
	}
	return C.GoBytes(cuidPtr, C.VIR_UUID_BUFLEN), nil
}

func (n *VirNetwork) GetUUIDString() (string, error) {
	var cUuid [C.VIR_UUID_STRING_BUFLEN](C.char)
	cuidPtr := unsafe.Pointer(&cUuid)
	result := C.virNetworkGetUUIDString(n.ptr, (*C.char)(cuidPtr))
	if result != 0 {
		return "", GetLastError()
	}
	return C.GoString((*C.char)(cuidPtr)), nil
}

func (n *VirNetwork) GetBridgeName() (string, error) {
	result := C.virNetworkGetBridgeName(n.ptr)
	if result == nil {
		return "", GetLastError()
	}
	bridge := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return bridge, nil
}

func (n *VirNetwork) GetXMLDesc(flags uint32) (string, error) {
	result := C.virNetworkGetXMLDesc(n.ptr, C.uint(flags))
	if result == nil {
		return "", GetLastError()
	}
	xml := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return xml, nil
}

func (n *VirNetwork) Undefine() error {
	result := C.virNetworkUndefine(n.ptr)
	if result == -1 {
		return GetLastError()
	}
	return nil
}
