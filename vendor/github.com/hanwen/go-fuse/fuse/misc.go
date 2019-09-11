// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Random odds and ends.

package fuse

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"syscall"
	"time"
	"unsafe"
)

func (code Status) String() string {
	if code <= 0 {
		return []string{
			"OK",
			"NOTIFY_POLL",
			"NOTIFY_INVAL_INODE",
			"NOTIFY_INVAL_ENTRY",
			"NOTIFY_STORE_CACHE",
			"NOTIFY_RETRIEVE_CACHE",
			"NOTIFY_DELETE",
		}[-code]
	}
	return fmt.Sprintf("%d=%v", int(code), syscall.Errno(code))
}

func (code Status) Ok() bool {
	return code == OK
}

// ToStatus extracts an errno number from Go error objects.  If it
// fails, it logs an error and returns ENOSYS.
func ToStatus(err error) Status {
	switch err {
	case nil:
		return OK
	case os.ErrPermission:
		return EPERM
	case os.ErrExist:
		return Status(syscall.EEXIST)
	case os.ErrNotExist:
		return ENOENT
	case os.ErrInvalid:
		return EINVAL
	}

	switch t := err.(type) {
	case syscall.Errno:
		return Status(t)
	case *os.SyscallError:
		return Status(t.Err.(syscall.Errno))
	case *os.PathError:
		return ToStatus(t.Err)
	case *os.LinkError:
		return ToStatus(t.Err)
	}
	log.Println("can't convert error type:", err)
	return ENOSYS
}

func toSlice(dest *[]byte, ptr unsafe.Pointer, byteCount uintptr) {
	h := (*reflect.SliceHeader)(unsafe.Pointer(dest))
	*h = reflect.SliceHeader{
		Data: uintptr(ptr),
		Len:  int(byteCount),
		Cap:  int(byteCount),
	}
}

func CurrentOwner() *Owner {
	return &Owner{
		Uid: uint32(os.Getuid()),
		Gid: uint32(os.Getgid()),
	}
}

const _UTIME_OMIT = ((1 << 30) - 2)

// UtimeToTimespec converts a "Time" pointer as passed to Utimens to a
// "Timespec" that can be passed to the utimensat syscall.
// A nil pointer is converted to the special UTIME_OMIT value.
func UtimeToTimespec(t *time.Time) (ts syscall.Timespec) {
	if t == nil {
		ts.Nsec = _UTIME_OMIT
	} else {
		ts = syscall.NsecToTimespec(t.UnixNano())
		// Go bug https://github.com/golang/go/issues/12777
		if ts.Nsec < 0 {
			ts.Nsec = 0
		}
	}
	return ts
}
