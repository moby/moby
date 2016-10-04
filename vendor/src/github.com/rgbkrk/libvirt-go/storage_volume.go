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

type VirStorageVol struct {
	ptr C.virStorageVolPtr
}

type VirStorageVolInfo struct {
	ptr C.virStorageVolInfo
}

func (v *VirStorageVol) Delete(flags uint32) error {
	result := C.virStorageVolDelete(v.ptr, C.uint(flags))
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (v *VirStorageVol) Free() error {
	if result := C.virStorageVolFree(v.ptr); result != 0 {
		return GetLastError()
	}
	return nil
}

func (v *VirStorageVol) GetInfo() (VirStorageVolInfo, error) {
	vi := VirStorageVolInfo{}
	var ptr C.virStorageVolInfo
	result := C.virStorageVolGetInfo(v.ptr, (*C.virStorageVolInfo)(unsafe.Pointer(&ptr)))
	if result == -1 {
		return vi, GetLastError()
	}
	vi.ptr = ptr
	return vi, nil
}

func (i *VirStorageVolInfo) GetType() int {
	return int(i.ptr._type)
}

func (i *VirStorageVolInfo) GetCapacityInBytes() uint64 {
	return uint64(i.ptr.capacity)
}

func (i *VirStorageVolInfo) GetAllocationInBytes() uint64 {
	return uint64(i.ptr.allocation)
}

func (v *VirStorageVol) GetKey() (string, error) {
	key := C.virStorageVolGetKey(v.ptr)
	if key == nil {
		return "", GetLastError()
	}
	return C.GoString(key), nil
}

func (v *VirStorageVol) GetName() (string, error) {
	name := C.virStorageVolGetName(v.ptr)
	if name == nil {
		return "", GetLastError()
	}
	return C.GoString(name), nil
}

func (v *VirStorageVol) GetPath() (string, error) {
	result := C.virStorageVolGetPath(v.ptr)
	if result == nil {
		return "", GetLastError()
	}
	path := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return path, nil
}

func (v *VirStorageVol) GetXMLDesc(flags uint32) (string, error) {
	result := C.virStorageVolGetXMLDesc(v.ptr, C.uint(flags))
	if result == nil {
		return "", GetLastError()
	}
	xml := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return xml, nil
}

func (v *VirStorageVol) Resize(capacity uint64, flags uint32) error {
	result := C.virStorageVolResize(v.ptr, C.ulonglong(capacity), C.uint(flags))
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (v *VirStorageVol) Wipe(flags uint32) error {
	result := C.virStorageVolWipe(v.ptr, C.uint(flags))
	if result == -1 {
		return GetLastError()
	}
	return nil
}
func (v *VirStorageVol) WipePattern(algorithm uint32, flags uint32) error {
	result := C.virStorageVolWipePattern(v.ptr, C.uint(algorithm), C.uint(flags))
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (v *VirStorageVol) Upload(stream *VirStream, offset, length uint64, flags uint32) error {
	if C.virStorageVolUpload(v.ptr, stream.ptr, C.ulonglong(offset),
		C.ulonglong(length), C.uint(flags)) == -1 {
		return GetLastError()
	}
	return nil
}

func (v *VirStorageVol) Download(stream *VirStream, offset, length uint64, flags uint32) error {
	if C.virStorageVolDownload(v.ptr, stream.ptr, C.ulonglong(offset),
		C.ulonglong(length), C.uint(flags)) == -1 {
		return GetLastError()
	}
	return nil
}

func (v *VirStorageVol) LookupPoolByVolume() (VirStoragePool, error) {
	poolPtr := C.virStoragePoolLookupByVolume(v.ptr)
	if poolPtr == nil {
		return VirStoragePool{}, GetLastError()
	}
	return VirStoragePool{ptr: poolPtr}, nil
}


