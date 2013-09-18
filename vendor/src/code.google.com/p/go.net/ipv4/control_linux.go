// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4

import (
	"os"
	"syscall"
	"unsafe"
)

// Linux provides a convenient path control option IP_PKTINFO that
// contains IP_SENDSRCADDR, IP_RECVDSTADDR, IP_RECVIF and IP_SENDIF.
const pktinfo = FlagSrc | FlagDst | FlagInterface

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
	if cf&pktinfo != 0 {
		if err := setIPv4PacketInfo(fd, on); err != nil {
			return err
		}
		if on {
			opt.set(cf & pktinfo)
		} else {
			opt.clear(cf & pktinfo)
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
	if opt.isset(pktinfo) {
		l += syscall.CmsgSpace(syscall.SizeofInet4Pktinfo)
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
		if opt.isset(pktinfo) {
			m := (*syscall.Cmsghdr)(unsafe.Pointer(&oob[off]))
			m.Level = ianaProtocolIP
			m.Type = syscall.IP_PKTINFO
			m.SetLen(syscall.CmsgLen(syscall.SizeofInet4Pktinfo))
			off += syscall.CmsgSpace(syscall.SizeofInet4Pktinfo)
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
		case syscall.IP_TTL:
			cm.TTL = int(*(*byte)(unsafe.Pointer(&m.Data[:1][0])))
		case syscall.IP_PKTINFO:
			pi := (*syscall.Inet4Pktinfo)(unsafe.Pointer(&m.Data[0]))
			cm.IfIndex = int(pi.Ifindex)
			cm.Dst = pi.Addr[:]
		}
	}
	return cm, nil
}

func marshalControlMessage(cm *ControlMessage) (oob []byte) {
	if cm == nil {
		return
	}
	l, off := 0, 0
	pion := false
	if cm.Src.To4() != nil || cm.IfIndex != 0 {
		pion = true
		l += syscall.CmsgSpace(syscall.SizeofInet4Pktinfo)
	}
	if l > 0 {
		oob = make([]byte, l)
		if pion {
			m := (*syscall.Cmsghdr)(unsafe.Pointer(&oob[off]))
			m.Level = ianaProtocolIP
			m.Type = syscall.IP_PKTINFO
			m.SetLen(syscall.CmsgLen(syscall.SizeofInet4Pktinfo))
			pi := (*syscall.Inet4Pktinfo)(unsafe.Pointer(&oob[off+syscall.CmsgLen(0)]))
			if ip := cm.Src.To4(); ip != nil {
				copy(pi.Addr[:], ip)
			}
			if cm.IfIndex != 0 {
				pi.Ifindex = int32(cm.IfIndex)
			}
			off += syscall.CmsgSpace(syscall.SizeofInet4Pktinfo)
		}
	}
	return
}
