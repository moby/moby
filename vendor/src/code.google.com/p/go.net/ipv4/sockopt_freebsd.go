// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4

import (
	"os"
	"syscall"
)

func ipv4SendSourceAddress(fd int) (bool, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_SENDSRCADDR)
	if err != nil {
		return false, os.NewSyscallError("getsockopt", err)
	}
	return v == 1, nil
}

func setIPv4SendSourceAddress(fd int, v bool) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_SENDSRCADDR, boolint(v)))
}
