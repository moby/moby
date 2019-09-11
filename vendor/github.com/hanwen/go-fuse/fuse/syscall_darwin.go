// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

// TODO - move these into Go's syscall package.

func sys_writev(fd int, iovecs *syscall.Iovec, cnt int) (n int, err error) {
	n1, _, e1 := syscall.Syscall(
		syscall.SYS_WRITEV,
		uintptr(fd), uintptr(unsafe.Pointer(iovecs)), uintptr(cnt))
	n = int(n1)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

func writev(fd int, packet [][]byte) (n int, err error) {
	iovecs := make([]syscall.Iovec, 0, len(packet))

	for _, v := range packet {
		if len(v) == 0 {
			continue
		}
		vec := syscall.Iovec{
			Base: &v[0],
		}
		vec.SetLen(len(v))
		iovecs = append(iovecs, vec)
	}

	sysErr := handleEINTR(func() error {
		var err error
		n, err = sys_writev(fd, &iovecs[0], len(iovecs))
		return err
	})
	if sysErr != nil {
		err = os.NewSyscallError("writev", sysErr)
	}
	return n, err
}

func getxattr(path string, attr string, dest []byte) (sz int, errno int) {
	pathBs := syscall.StringBytePtr(path)
	attrBs := syscall.StringBytePtr(attr)
	size, _, errNo := syscall.Syscall6(
		syscall.SYS_GETXATTR,
		uintptr(unsafe.Pointer(pathBs)),
		uintptr(unsafe.Pointer(attrBs)),
		uintptr(unsafe.Pointer(&dest[0])),
		uintptr(len(dest)),
		0, 0)
	return int(size), int(errNo)
}

func GetXAttr(path string, attr string, dest []byte) (value []byte, errno int) {
	sz, errno := getxattr(path, attr, dest)

	for sz > cap(dest) && errno == 0 {
		dest = make([]byte, sz)
		sz, errno = getxattr(path, attr, dest)
	}

	if errno != 0 {
		return nil, errno
	}

	return dest[:sz], errno
}

func listxattr(path string, dest []byte) (sz int, errno int) {
	pathbs := syscall.StringBytePtr(path)
	var destPointer unsafe.Pointer
	if len(dest) > 0 {
		destPointer = unsafe.Pointer(&dest[0])
	}
	size, _, errNo := syscall.Syscall(
		syscall.SYS_LISTXATTR,
		uintptr(unsafe.Pointer(pathbs)),
		uintptr(destPointer),
		uintptr(len(dest)))

	return int(size), int(errNo)
}

func ListXAttr(path string) (attributes []string, errno int) {
	dest := make([]byte, 0)
	sz, errno := listxattr(path, dest)
	if errno != 0 {
		return nil, errno
	}

	for sz > cap(dest) && errno == 0 {
		dest = make([]byte, sz)
		sz, errno = listxattr(path, dest)
	}

	// -1 to drop the final empty slice.
	dest = dest[:sz-1]
	attributesBytes := bytes.Split(dest, []byte{0})
	attributes = make([]string, len(attributesBytes))
	for i, v := range attributesBytes {
		attributes[i] = string(v)
	}
	return attributes, errno
}

func Setxattr(path string, attr string, data []byte, flags int) (errno int) {
	pathbs := syscall.StringBytePtr(path)
	attrbs := syscall.StringBytePtr(attr)
	_, _, errNo := syscall.Syscall6(
		syscall.SYS_SETXATTR,
		uintptr(unsafe.Pointer(pathbs)),
		uintptr(unsafe.Pointer(attrbs)),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(flags), 0)

	return int(errNo)
}

func Removexattr(path string, attr string) (errno int) {
	pathbs := syscall.StringBytePtr(path)
	attrbs := syscall.StringBytePtr(attr)
	_, _, errNo := syscall.Syscall(
		syscall.SYS_REMOVEXATTR,
		uintptr(unsafe.Pointer(pathbs)),
		uintptr(unsafe.Pointer(attrbs)), 0)
	return int(errNo)
}
