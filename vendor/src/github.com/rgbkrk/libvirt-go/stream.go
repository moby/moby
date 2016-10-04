package libvirt

/*
#cgo LDFLAGS: -lvirt 
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>
*/
import "C"
import (
	"io"
	"unsafe"
)

type VirStream struct {
	ptr C.virStreamPtr
}

func NewVirStream(c *VirConnection, flags uint) (*VirStream, error) {
	virStream := C.virStreamNew(c.ptr, C.uint(flags))
	if virStream == nil {
		return nil, GetLastError()
	}

	return &VirStream{
		ptr: virStream,
	}, nil
}

func (v *VirStream) Abort() error {
	result := C.virStreamAbort(v.ptr)
	if result == -1 {
		return GetLastError()
	}

	return nil
}

func (v *VirStream) Close() error {
	result := C.virStreamFinish(v.ptr)
	if result == -1 {
		return GetLastError()
	}

	return nil
}

func (v *VirStream) Free() error {
	result := C.virStreamFree(v.ptr)
	if result == -1 {
		return GetLastError()
	}

	return nil
}

func (v *VirStream) Read(p []byte) (int, error) {
	n := C.virStreamRecv(v.ptr, (*C.char)(unsafe.Pointer(&p[0])), C.size_t(len(p)))
	if n < 0 {
		return 0, GetLastError()
	}
	if n == 0 {
		return 0, io.EOF
	}

	return int(n), nil
}

func (v *VirStream) Write(p []byte) (int, error) {
	n := C.virStreamSend(v.ptr, (*C.char)(unsafe.Pointer(&p[0])), C.size_t(len(p)))
	if n < 0 {
		return 0, GetLastError()
	}
	if n == 0 {
		return 0, io.EOF
	}

	return int(n), nil
}
