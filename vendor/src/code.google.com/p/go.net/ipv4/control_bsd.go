// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd netbsd openbsd

package ipv4

import (
	"net"
	"os"
	"syscall"
	"unsafe"
)

func setControlMessage(fd int, opt *rawOpt, cf ControlFlags, on bool) error {
	opt.Lock()
	defer opt.Unlock()
	if cf&FlagTTL != 0 {
		if err := setIPv4ReceiveTTL(fd, on); err != nil {
			return err
		}
		if on {
			opt.set(FlagTTL)
		} else {
			opt.clear(FlagTTL)
		}
	}
	if cf&FlagDst != 0 {
		if err := setIPv4ReceiveDestinationAddress(fd, on); err != nil {
			return err
		}
		if on {
			opt.set(FlagDst)
		} else {
			opt.clear(FlagDst)
		}
	}
	if cf&FlagInterface != 0 {
		if err := setIPv4ReceiveInterface(fd, on); err != nil {
			return err
		}
		if on {
			opt.set(FlagInterface)
		} else {
			opt.clear(FlagInterface)
		}
	}
	return nil
}

func newControlMessage(opt *rawOpt) (oob []byte) {
	opt.Lock()
	defer opt.Unlock()
	l, off := 0, 0
	if opt.isset(FlagTTL) {
		l += syscall.CmsgSpace(1)
	}
	if opt.isset(FlagDst) {
		l += syscall.CmsgSpace(net.IPv4len)
	}
	if opt.isset(FlagInterface) {
		l += syscall.CmsgSpace(syscall.SizeofSockaddrDatalink)
	}
	if l > 0 {
		oob = make([]byte, l)
		if opt.isset(FlagTTL) {
			m := (*syscall.Cmsghdr)(unsafe.Pointer(&oob[off]))
			m.Level = ianaProtocolIP
			m.Type = syscall.IP_RECVTTL
			m.SetLen(syscall.CmsgLen(1))
			off += syscall.CmsgSpace(1)
		}
		if opt.isset(FlagDst) {
			m := (*syscall.Cmsghdr)(unsafe.Pointer(&oob[off]))
			m.Level = ianaProtocolIP
			m.Type = syscall.IP_RECVDSTADDR
			m.SetLen(syscall.CmsgLen(net.IPv4len))
			off += syscall.CmsgSpace(net.IPv4len)
		}
		if opt.isset(FlagInterface) {
			m := (*syscall.Cmsghdr)(unsafe.Pointer(&oob[off]))
			m.Level = ianaProtocolIP
			m.Type = syscall.IP_RECVIF
			m.SetLen(syscall.CmsgLen(syscall.SizeofSockaddrDatalink))
			off += syscall.CmsgSpace(syscall.SizeofSockaddrDatalink)
		}
	}
	return
}

func parseControlMessage(b []byte) (*ControlMessage, error) {
	if len(b) == 0 {
		return nil, nil
	}
	cmsgs, err := syscall.ParseSocketControlMessage(b)
	if err != nil {
		return nil, os.NewSyscallError("parse socket control message", err)
	}
	cm := &ControlMessage{}
	for _, m := range cmsgs {
		if m.Header.Level != ianaProtocolIP {
			continue
		}
		switch m.Header.Type {
		case syscall.IP_RECVTTL:
			cm.TTL = int(*(*byte)(unsafe.Pointer(&m.Data[:1][0])))
		case syscall.IP_RECVDSTADDR:
			cm.Dst = m.Data[:net.IPv4len]
		case syscall.IP_RECVIF:
			sadl := (*syscall.SockaddrDatalink)(unsafe.Pointer(&m.Data[0]))
			cm.IfIndex = int(sadl.Index)
		}
	}
	return cm, nil
}

func marshalControlMessage(cm *ControlMessage) []byte {
	// TODO(mikio): Implement IP_PKTINFO stuff when OS X 10.8 comes
	return nil
}
