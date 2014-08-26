// +build linux

package audit

/*
  The audit package is a go bindings to libaudit that only allows for
  logging audit events.

  Author Steve Grubb <sgrubb@redhat.com>

*/

// #cgo pkg-config: audit
// #include "libaudit.h"
// #include <unistd.h>
// #include <stdlib.h>
// #include <string.h>
// #include <stdio.h>
import "C"

import (
	"unsafe"
)

const (
	AUDIT_VIRT_CONTROL    = 2500
	AUDIT_VIRT_RESOURCE   = 2501
	AUDIT_VIRT_MACHINE_ID = 2502
)

// type=VIRT_CONTROL msg=audit(08/05/2014 17:01:05.891:6471) : pid=1265 uid=root auid=unset ses=unset subj=system_u:system_r:virtd_t:s0-s0:c0.c1023 msg='virt=kvm op=start reason=booted vm=vm1 uuid=462dcd6d-fb68-4a26-a96f-56eb024515b9 vm-pid=22527 exe=/usr/sbin/libvirtd hostname=? addr=? terminal=? res=success'

func AuditValueNeedsEncoding(str string) bool {
	cstr := C.CString(str)
	defer C.free(unsafe.Pointer(cstr))
	len := C.strlen(cstr)

	res, _ := C.audit_value_needs_encoding(cstr, C.uint(len))
	if res != 0 {
		return true
	}
	return false
}

func AuditEncodeNVString(name string, value string) string {
	cname := C.CString(name)
	cval := C.CString(value)

	cres := C.audit_encode_nv_string(cname, cval, 0)

	C.free(unsafe.Pointer(cname))
	C.free(unsafe.Pointer(cval))
	defer C.free(unsafe.Pointer(cres))

	return C.GoString(cres)
}

func AuditLogUserEvent(event_type int, message string, result bool) error {
	var r int
	fd := C.audit_open()
	if result {
		r = 1
	} else {
		r = 0
	}
	if fd > 0 {
		cmsg := C.CString(message)
		_, err := C.audit_log_user_message(fd, C.int(event_type), cmsg, nil, nil, nil, C.int(r))
		C.free(unsafe.Pointer(cmsg))
		C.close(fd)
		return err
	}
	return nil
}
