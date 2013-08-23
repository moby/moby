// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4

import (
	"net"
	"syscall"
)

func (c *dgramOpt) MulticastTTL() (int, error) {
	// TODO(mikio): Implement this
	return 0, syscall.EPLAN9
}

func (c *dgramOpt) SetMulticastTTL(ttl int) error {
	// TODO(mikio): Implement this
	return syscall.EPLAN9
}

func (c *dgramOpt) MulticastInterface() (*net.Interface, error) {
	// TODO(mikio): Implement this
	return nil, syscall.EPLAN9
}

func (c *dgramOpt) SetMulticastInterface(ifi *net.Interface) error {
	// TODO(mikio): Implement this
	return syscall.EPLAN9
}

func (c *dgramOpt) MulticastLoopback() (bool, error) {
	// TODO(mikio): Implement this
	return false, syscall.EPLAN9
}

func (c *dgramOpt) SetMulticastLoopback(on bool) error {
	// TODO(mikio): Implement this
	return syscall.EPLAN9
}

func (c *dgramOpt) JoinGroup(ifi *net.Interface, grp net.Addr) error {
	// TODO(mikio): Implement this
	return syscall.EPLAN9
}

func (c *dgramOpt) LeaveGroup(ifi *net.Interface, grp net.Addr) error {
	// TODO(mikio): Implement this
	return syscall.EPLAN9
}
