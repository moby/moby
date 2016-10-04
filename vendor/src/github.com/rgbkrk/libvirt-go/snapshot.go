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

type VirDomainSnapshot struct {
	ptr C.virDomainSnapshotPtr
}

func (s *VirDomainSnapshot) Free() error {
	if result := C.virDomainSnapshotFree(s.ptr); result != 0 {
		return GetLastError()
	}
	return nil
}

func (s *VirDomainSnapshot) Delete(flags uint32) error {
	result := C.virDomainSnapshotDelete(s.ptr, C.uint(flags))
	if result != 0 {
		return GetLastError()
	}
	return nil
}

func (s *VirDomainSnapshot) RevertToSnapshot(flags uint32) error {
	result := C.virDomainRevertToSnapshot(s.ptr, C.uint(flags))
	if result != 0 {
		return GetLastError()
	}
	return nil
}

func (d *VirDomain) CreateSnapshotXML(xml string, flags uint32) (VirDomainSnapshot, error) {
	cXml := C.CString(xml)
	defer C.free(unsafe.Pointer(cXml))
	result := C.virDomainSnapshotCreateXML(d.ptr, cXml, C.uint(flags))
	if result == nil {
		return VirDomainSnapshot{}, GetLastError()
	}
	return VirDomainSnapshot{ptr: result}, nil
}

func (d *VirDomain) Save(destFile string) error {
	cPath := C.CString(destFile)
	defer C.free(unsafe.Pointer(cPath))
	result := C.virDomainSave(d.ptr, cPath)
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (d *VirDomain) SaveFlags(destFile string, destXml string, flags uint32) error {
	cDestFile := C.CString(destFile)
	cDestXml := C.CString(destXml)
	defer C.free(unsafe.Pointer(cDestXml))
	defer C.free(unsafe.Pointer(cDestFile))
	result := C.virDomainSaveFlags(d.ptr, cDestFile, cDestXml, C.uint(flags))
	if result == -1 {
		return GetLastError()
	}
	return nil
}

func (conn VirConnection) Restore(srcFile string) error {
	cPath := C.CString(srcFile)
	defer C.free(unsafe.Pointer(cPath))
	if result := C.virDomainRestore(conn.ptr, cPath); result == -1 {
		return GetLastError()
	}
	return nil
}

func (conn VirConnection) RestoreFlags(srcFile, xmlConf string, flags uint32) error {
	cPath := C.CString(srcFile)
	defer C.free(unsafe.Pointer(cPath))
	var cXmlConf *C.char
	if xmlConf != "" {
		cXmlConf = C.CString(xmlConf)
		defer C.free(unsafe.Pointer(cXmlConf))
	}
	if result := C.virDomainRestoreFlags(conn.ptr, cPath, cXmlConf, C.uint(flags)); result == -1 {
		return GetLastError()
	}
	return nil
}
