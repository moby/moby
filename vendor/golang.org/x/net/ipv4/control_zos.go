// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4

import (
	"net"
	"unsafe"

	"golang.org/x/net/internal/iana"
	"golang.org/x/net/internal/socket"
)

func marshalPacketInfo(b []byte, cm *ControlMessage) []byte {
	m := socket.ControlMessage(b)
	m.MarshalHeader(iana.ProtocolIP, sysIP_PKTINFO, sizeofInetPktinfo)
	if cm != nil {
		pi := (*inetPktinfo)(unsafe.Pointer(&m.Data(sizeofInetPktinfo)[0]))
		if ip := cm.Src.To4(); ip != nil {
			copy(pi.Addr[:], ip)
		}
		if cm.IfIndex > 0 {
			pi.setIfindex(cm.IfIndex)
		}
	}
	return m.Next(sizeofInetPktinfo)
}

func parsePacketInfo(cm *ControlMessage, b []byte) {
	pi := (*inetPktinfo)(unsafe.Pointer(&b[0]))
	cm.IfIndex = int(pi.Ifindex)
	if len(cm.Dst) < net.IPv4len {
		cm.Dst = make(net.IP, net.IPv4len)
	}
	copy(cm.Dst, pi.Addr[:])
}

func setControlMessage(c *socket.Conn, opt *rawOpt, cf ControlFlags, on bool) error {
	opt.Lock()
	defer opt.Unlock()
	if so, ok := sockOpts[ssoReceiveTTL]; ok && cf&FlagTTL != 0 {
		if err := so.SetInt(c, boolint(on)); err != nil {
			return err
		}
		if on {
			opt.set(FlagTTL)
		} else {
			opt.clear(FlagTTL)
		}
	}
	if so, ok := sockOpts[ssoPacketInfo]; ok {
		if cf&(FlagSrc|FlagDst|FlagInterface) != 0 {
			if err := so.SetInt(c, boolint(on)); err != nil {
				return err
			}
			if on {
				opt.set(cf & (FlagSrc | FlagDst | FlagInterface))
			} else {
				opt.clear(cf & (FlagSrc | FlagDst | FlagInterface))
			}
		}
	} else {
		if so, ok := sockOpts[ssoReceiveDst]; ok && cf&FlagDst != 0 {
			if err := so.SetInt(c, boolint(on)); err != nil {
				return err
			}
			if on {
				opt.set(FlagDst)
			} else {
				opt.clear(FlagDst)
			}
		}
		if so, ok := sockOpts[ssoReceiveInterface]; ok && cf&FlagInterface != 0 {
			if err := so.SetInt(c, boolint(on)); err != nil {
				return err
			}
			if on {
				opt.set(FlagInterface)
			} else {
				opt.clear(FlagInterface)
			}
		}
	}
	return nil
}
