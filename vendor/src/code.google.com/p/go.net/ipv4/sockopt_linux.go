// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4

import (
	"net"
	"os"
	"syscall"
)

func ipv4ReceiveTOS(fd int) (bool, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_RECVTOS)
	if err != nil {
		return false, os.NewSyscallError("getsockopt", err)
	}
	return v == 1, nil
}

func setIPv4ReceiveTOS(fd int, v bool) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_RECVTOS, boolint(v)))
}

func ipv4MulticastTTL(fd int) (int, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_MULTICAST_TTL)
	if err != nil {
		return 0, os.NewSyscallError("getsockopt", err)
	}
	return v, nil
}

func setIPv4MulticastTTL(fd int, v int) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_MULTICAST_TTL, v))
}

func ipv4PacketInfo(fd int) (bool, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_PKTINFO)
	if err != nil {
		return false, os.NewSyscallError("getsockopt", err)
	}
	return v == 1, nil
}

func setIPv4PacketInfo(fd int, v bool) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_PKTINFO, boolint(v)))
}

func ipv4MulticastInterface(fd int) (*net.Interface, error) {
	mreqn, err := syscall.GetsockoptIPMreqn(fd, ianaProtocolIP, syscall.IP_MULTICAST_IF)
	if err != nil {
		return nil, os.NewSyscallError("getsockopt", err)
	}
	if int(mreqn.Ifindex) == 0 {
		return nil, nil
	}
	return net.InterfaceByIndex(int(mreqn.Ifindex))
}

func setIPv4MulticastInterface(fd int, ifi *net.Interface) error {
	mreqn := syscall.IPMreqn{}
	if ifi != nil {
		mreqn.Ifindex = int32(ifi.Index)
	}
	return os.NewSyscallError("setsockopt", syscall.SetsockoptIPMreqn(fd, ianaProtocolIP, syscall.IP_MULTICAST_IF, &mreqn))
}

func ipv4MulticastLoopback(fd int) (bool, error) {
	v, err := syscall.GetsockoptInt(fd, ianaProtocolIP, syscall.IP_MULTICAST_LOOP)
	if err != nil {
		return false, os.NewSyscallError("getsockopt", err)
	}
	return v == 1, nil
}

func setIPv4MulticastLoopback(fd int, v bool) error {
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, ianaProtocolIP, syscall.IP_MULTICAST_LOOP, boolint(v)))
}

func joinIPv4Group(fd int, ifi *net.Interface, grp net.IP) error {
	mreqn := syscall.IPMreqn{Multiaddr: [4]byte{grp[0], grp[1], grp[2], grp[3]}}
	if ifi != nil {
		mreqn.Ifindex = int32(ifi.Index)
	}
	return os.NewSyscallError("setsockopt", syscall.SetsockoptIPMreqn(fd, ianaProtocolIP, syscall.IP_ADD_MEMBERSHIP, &mreqn))
}

func leaveIPv4Group(fd int, ifi *net.Interface, grp net.IP) error {
	mreqn := syscall.IPMreqn{Multiaddr: [4]byte{grp[0], grp[1], grp[2], grp[3]}}
	if ifi != nil {
		mreqn.Ifindex = int32(ifi.Index)
	}
	return os.NewSyscallError("setsockopt", syscall.SetsockoptIPMreqn(fd, ianaProtocolIP, syscall.IP_DROP_MEMBERSHIP, &mreqn))
}
