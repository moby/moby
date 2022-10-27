// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows && !zos
// +build !aix,!darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!windows,!zos

package socket

import "net"

const (
	sysAF_UNSPEC = 0x0
	sysAF_INET   = 0x2
	sysAF_INET6  = 0xa

	sysSOCK_RAW = 0x3

	sizeofSockaddrInet4 = 0x10
	sizeofSockaddrInet6 = 0x1c
)

func marshalInetAddr(ip net.IP, port int, zone string) []byte {
	return nil
}

func parseInetAddr(b []byte, network string) (net.Addr, error) {
	return nil, errNotImplemented
}

func getsockopt(s uintptr, level, name int, b []byte) (int, error) {
	return 0, errNotImplemented
}

func setsockopt(s uintptr, level, name int, b []byte) error {
	return errNotImplemented
}

func recvmsg(s uintptr, buffers [][]byte, oob []byte, flags int, network string) (n, oobn int, recvflags int, from net.Addr, err error) {
	return 0, 0, 0, nil, errNotImplemented
}

func sendmsg(s uintptr, buffers [][]byte, oob []byte, to net.Addr, flags int) (int, error) {
	return 0, errNotImplemented
}

func recvmmsg(s uintptr, hs []mmsghdr, flags int) (int, error) {
	return 0, errNotImplemented
}

func sendmmsg(s uintptr, hs []mmsghdr, flags int) (int, error) {
	return 0, errNotImplemented
}
