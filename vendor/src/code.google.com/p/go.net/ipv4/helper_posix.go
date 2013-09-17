// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd windows

package ipv4

import (
	"bytes"
	"net"
	"syscall"
)

func setSyscallIPMreq(mreq *syscall.IPMreq, ifi *net.Interface) error {
	if ifi == nil {
		return nil
	}
	ifat, err := ifi.Addrs()
	if err != nil {
		return err
	}
	for _, ifa := range ifat {
		switch v := ifa.(type) {
		case *net.IPAddr:
			if a := v.IP.To4(); a != nil {
				copy(mreq.Interface[:], a)
				goto done
			}
		case *net.IPNet:
			if a := v.IP.To4(); a != nil {
				copy(mreq.Interface[:], a)
				goto done
			}
		}
	}
done:
	if bytes.Equal(mreq.Multiaddr[:], net.IPv4zero.To4()) {
		return errNoSuchMulticastInterface
	}
	return nil
}
