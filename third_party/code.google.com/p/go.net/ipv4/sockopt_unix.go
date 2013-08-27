// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd

package ipv4

import (
	"os"
	"syscall"
)

func ipv4TOS(fd int) (int, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_TOS)
	if err != nil {
		return 0, os.NewSyscallError("getsockopt", err)
	}
	return v, nil
}

func setIPv4TOS(fd int, v int) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_TOS, v))
}

func ipv4TTL(fd int) (int, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_TTL)
	if err != nil {
		return 0, os.NewSyscallError("getsockopt", err)
	}
	return v, nil
}

func setIPv4TTL(fd int, v int) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_TTL, v))
}

func ipv4ReceiveTTL(fd int) (bool, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_RECVTTL)
	if err != nil {
		return false, os.NewSyscallError("getsockopt", err)
	}
	return v == 1, nil
}

func setIPv4ReceiveTTL(fd int, v bool) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_RECVTTL, boolint(v)))
}

func ipv4HeaderPrepend(fd int) (bool, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_HDRINCL)
	if err != nil {
		return false, os.NewSyscallError("getsockopt", err)
	}
	return v == 1, nil
}

func setIPv4HeaderPrepend(fd int, v bool) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_HDRINCL, boolint(v)))
}
