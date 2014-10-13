package libvirt

/*
#cgo LDFLAGS: -lvirt -ldl
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"
)

type VirNodeInfo struct {
	ptr C.virNodeInfo
}

func (ni *VirNodeInfo) GetModel() string {
	model := C.GoString((*C.char)(unsafe.Pointer(&ni.ptr.model)))
	return model
}

func (ni *VirNodeInfo) GetMemoryKB() uint64 {
	return uint64(ni.ptr.memory)
}

func (ni *VirNodeInfo) GetCPUs() uint32 {
	return uint32(ni.ptr.cpus)
}

func (ni *VirNodeInfo) GetMhz() uint32 {
	return uint32(ni.ptr.mhz)
}

func (ni *VirNodeInfo) GetNodes() uint32 {
	return uint32(ni.ptr.nodes)
}

func (ni *VirNodeInfo) GetSockets() uint32 {
	return uint32(ni.ptr.sockets)
}

func (ni *VirNodeInfo) GetCores() uint32 {
	return uint32(ni.ptr.cores)
}

func (ni *VirNodeInfo) GetThreads() uint32 {
	return uint32(ni.ptr.threads)
}
