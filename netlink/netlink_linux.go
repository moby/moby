package netlink

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

var nextSeqNr int

func nativeEndian() binary.ByteOrder {
	var x uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func getSeq() int {
	nextSeqNr = nextSeqNr + 1
	return nextSeqNr
}

func getIpFamily(ip net.IP) int {
	if len(ip) <= net.IPv4len {
		return syscall.AF_INET
	}
	if ip.To4() != nil {
		return syscall.AF_INET
	}
	return syscall.AF_INET6
}

type NetlinkRequestData interface {
	ToWireFormat() []byte
}

type IfInfomsg struct {
	syscall.IfInfomsg
}

func newIfInfomsg(family int) *IfInfomsg {
	msg := &IfInfomsg{}
	msg.Family = uint8(family)
	msg.Type = uint16(0)
	msg.Index = int32(0)
	msg.Flags = uint32(0)
	msg.Change = uint32(0)

	return msg
}

func (msg *IfInfomsg) ToWireFormat() []byte {
	native := nativeEndian()

	len := syscall.SizeofIfInfomsg
	b := make([]byte, len)
	b[0] = msg.Family
	b[1] = 0
	native.PutUint16(b[2:4], msg.Type)
	native.PutUint32(b[4:8], uint32(msg.Index))
	native.PutUint32(b[8:12], msg.Flags)
	native.PutUint32(b[12:16], msg.Change)
	return b
}

type IfAddrmsg struct {
	syscall.IfAddrmsg
}

func newIfAddrmsg(family int) *IfAddrmsg {
	msg := &IfAddrmsg{}
	msg.Family = uint8(family)
	msg.Prefixlen = uint8(0)
	msg.Flags = uint8(0)
	msg.Scope = uint8(0)
	msg.Index = uint32(0)

	return msg
}

func (msg *IfAddrmsg) ToWireFormat() []byte {
	native := nativeEndian()

	len := syscall.SizeofIfAddrmsg
	b := make([]byte, len)
	b[0] = msg.Family
	b[1] = msg.Prefixlen
	b[2] = msg.Flags
	b[3] = msg.Scope
	native.PutUint32(b[4:8], msg.Index)
	return b
}

type RtMsg struct {
	syscall.RtMsg
}

func newRtMsg(family int) *RtMsg {
	msg := &RtMsg{}
	msg.Family = uint8(family)
	msg.Table = syscall.RT_TABLE_MAIN
	msg.Scope = syscall.RT_SCOPE_UNIVERSE
	msg.Protocol = syscall.RTPROT_BOOT
	msg.Type = syscall.RTN_UNICAST

	return msg
}

func (msg *RtMsg) ToWireFormat() []byte {
	native := nativeEndian()

	len := syscall.SizeofRtMsg
	b := make([]byte, len)
	b[0] = msg.Family
	b[1] = msg.Dst_len
	b[2] = msg.Src_len
	b[3] = msg.Tos
	b[4] = msg.Table
	b[5] = msg.Protocol
	b[6] = msg.Scope
	b[7] = msg.Type
	native.PutUint32(b[8:12], msg.Flags)
	return b
}

func rtaAlignOf(attrlen int) int {
	return (attrlen + syscall.RTA_ALIGNTO - 1) & ^(syscall.RTA_ALIGNTO - 1)
}

type RtAttr struct {
	syscall.RtAttr
	Data []byte
}

func newRtAttr(attrType int, data []byte) *RtAttr {
	attr := &RtAttr{}
	attr.Type = uint16(attrType)
	attr.Data = data

	return attr
}

func (attr *RtAttr) ToWireFormat() []byte {
	native := nativeEndian()

	len := syscall.SizeofRtAttr + len(attr.Data)
	b := make([]byte, rtaAlignOf(len))
	native.PutUint16(b[0:2], uint16(len))
	native.PutUint16(b[2:4], attr.Type)
	for i, d := range attr.Data {
		b[4+i] = d
	}

	return b
}

type NetlinkRequest struct {
	syscall.NlMsghdr
	Data []NetlinkRequestData
}

func (rr *NetlinkRequest) ToWireFormat() []byte {
	native := nativeEndian()

	length := rr.Len
	dataBytes := make([][]byte, len(rr.Data))
	for i, data := range rr.Data {
		dataBytes[i] = data.ToWireFormat()
		length = length + uint32(len(dataBytes[i]))
	}
	b := make([]byte, length)
	native.PutUint32(b[0:4], length)
	native.PutUint16(b[4:6], rr.Type)
	native.PutUint16(b[6:8], rr.Flags)
	native.PutUint32(b[8:12], rr.Seq)
	native.PutUint32(b[12:16], rr.Pid)

	i := 16
	for _, data := range dataBytes {
		for _, dataByte := range data {
			b[i] = dataByte
			i = i + 1
		}
	}
	return b
}

func (rr *NetlinkRequest) AddData(data NetlinkRequestData) {
	rr.Data = append(rr.Data, data)
}

func newNetlinkRequest(proto, flags int) *NetlinkRequest {
	rr := &NetlinkRequest{}
	rr.Len = uint32(syscall.NLMSG_HDRLEN)
	rr.Type = uint16(proto)
	rr.Flags = syscall.NLM_F_REQUEST | uint16(flags)
	rr.Seq = uint32(getSeq())
	return rr
}

type NetlinkSocket struct {
	fd  int
	lsa syscall.SockaddrNetlink
}

func getNetlinkSocket() (*NetlinkSocket, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	s := &NetlinkSocket{
		fd: fd,
	}
	s.lsa.Family = syscall.AF_NETLINK
	if err := syscall.Bind(fd, &s.lsa); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return s, nil
}

func (s *NetlinkSocket) Close() {
	syscall.Close(s.fd)
}

func (s *NetlinkSocket) Send(request *NetlinkRequest) error {
	if err := syscall.Sendto(s.fd, request.ToWireFormat(), 0, &s.lsa); err != nil {
		return err
	}
	return nil
}

func (s *NetlinkSocket) Recieve() ([]syscall.NetlinkMessage, error) {
	rb := make([]byte, syscall.Getpagesize())
	nr, _, err := syscall.Recvfrom(s.fd, rb, 0)
	if err != nil {
		return nil, err
	}
	if nr < syscall.NLMSG_HDRLEN {
		return nil, fmt.Errorf("Got short response from netlink")
	}
	rb = rb[:nr]
	return syscall.ParseNetlinkMessage(rb)
}

func (s *NetlinkSocket) GetPid() (uint32, error) {
	lsa, err := syscall.Getsockname(s.fd)
	if err != nil {
		return 0, err
	}
	switch v := lsa.(type) {
	case *syscall.SockaddrNetlink:
		return v.Pid, nil
	}
	return 0, fmt.Errorf("Wrong socket type")
}

func (s *NetlinkSocket) HandleAck(seq uint32) error {
	native := nativeEndian()

	pid, err := s.GetPid()
	if err != nil {
		return err
	}

done:
	for {
		msgs, err := s.Recieve()
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if m.Header.Seq != seq {
				return fmt.Errorf("Wrong Seq nr %d, expected %d", m.Header.Seq, seq)
			}
			if m.Header.Pid != pid {
				return fmt.Errorf("Wrong pid %d, expected %d", m.Header.Pid, pid)
			}
			if m.Header.Type == syscall.NLMSG_DONE {
				break done
			}
			if m.Header.Type == syscall.NLMSG_ERROR {
				error := int32(native.Uint32(m.Data[0:4]))
				if error == 0 {
					break done
				}
				return syscall.Errno(-error)
			}
		}
	}

	return nil
}

// Add a new default gateway. Identical to:
// ip route add default via $ip
func AddDefaultGw(ip net.IP) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	family := getIpFamily(ip)

	wb := newNetlinkRequest(syscall.RTM_NEWROUTE, syscall.NLM_F_CREATE|syscall.NLM_F_EXCL|syscall.NLM_F_ACK)

	msg := newRtMsg(family)
	wb.AddData(msg)

	var ipData []byte
	if family == syscall.AF_INET {
		ipData = ip.To4()
	} else {
		ipData = ip.To16()
	}

	gateway := newRtAttr(syscall.RTA_GATEWAY, ipData)

	wb.AddData(gateway)

	if err := s.Send(wb); err != nil {
		return err
	}

	return s.HandleAck(wb.Seq)

}

// Bring up a particular network interface
func NetworkLinkUp(iface *net.Interface) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_NEWLINK, syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	msg.Change = syscall.IFF_UP
	msg.Flags = syscall.IFF_UP
	msg.Index = int32(iface.Index)
	wb.AddData(msg)

	if err := s.Send(wb); err != nil {
		return err
	}

	return s.HandleAck(wb.Seq)
}

// Add an Ip address to an interface. This is identical to:
// ip addr add $ip/$ipNet dev $iface
func NetworkLinkAddIp(iface *net.Interface, ip net.IP, ipNet *net.IPNet) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	family := getIpFamily(ip)

	wb := newNetlinkRequest(syscall.RTM_NEWADDR, syscall.NLM_F_CREATE|syscall.NLM_F_EXCL|syscall.NLM_F_ACK)

	msg := newIfAddrmsg(family)
	msg.Index = uint32(iface.Index)
	prefixLen, _ := ipNet.Mask.Size()
	msg.Prefixlen = uint8(prefixLen)
	wb.AddData(msg)

	var ipData []byte
	if family == syscall.AF_INET {
		ipData = ip.To4()
	} else {
		ipData = ip.To16()
	}

	localData := newRtAttr(syscall.IFA_LOCAL, ipData)
	wb.AddData(localData)

	addrData := newRtAttr(syscall.IFA_ADDRESS, ipData)
	wb.AddData(addrData)

	if err := s.Send(wb); err != nil {
		return err
	}

	return s.HandleAck(wb.Seq)
}

func zeroTerminated(s string) []byte {
	bytes := make([]byte, len(s)+1)
	for i := 0; i < len(s); i++ {
		bytes[i] = s[i]
	}
	bytes[len(s)] = 0
	return bytes
}

func nonZeroTerminated(s string) []byte {
	bytes := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		bytes[i] = s[i]
	}
	return bytes
}

// Add a new network link of a specified type. This is identical to
// running: ip add link $name type $linkType
func NetworkLinkAdd(name string, linkType string) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_NEWLINK, syscall.NLM_F_CREATE|syscall.NLM_F_EXCL|syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	wb.AddData(msg)

	nameData := newRtAttr(syscall.IFLA_IFNAME, zeroTerminated(name))
	wb.AddData(nameData)

	IFLA_INFO_KIND := 1

	kindData := newRtAttr(IFLA_INFO_KIND, nonZeroTerminated(linkType))

	infoData := newRtAttr(syscall.IFLA_LINKINFO, kindData.ToWireFormat())
	wb.AddData(infoData)

	if err := s.Send(wb); err != nil {
		return err
	}

	return s.HandleAck(wb.Seq)
}

// Returns an array of IPNet for all the currently routed subnets on ipv4
// This is similar to the first column of "ip route" output
func NetworkGetRoutes() ([]*net.IPNet, error) {
	native := nativeEndian()

	s, err := getNetlinkSocket()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_GETROUTE, syscall.NLM_F_DUMP)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	wb.AddData(msg)

	if err := s.Send(wb); err != nil {
		return nil, err
	}

	pid, err := s.GetPid()
	if err != nil {
		return nil, err
	}

	res := make([]*net.IPNet, 0)

done:
	for {
		msgs, err := s.Recieve()
		if err != nil {
			return nil, err
		}
		for _, m := range msgs {
			if m.Header.Seq != wb.Seq {
				return nil, fmt.Errorf("Wrong Seq nr %d, expected 1", m.Header.Seq)
			}
			if m.Header.Pid != pid {
				return nil, fmt.Errorf("Wrong pid %d, expected %d", m.Header.Pid, pid)
			}
			if m.Header.Type == syscall.NLMSG_DONE {
				break done
			}
			if m.Header.Type == syscall.NLMSG_ERROR {
				error := int32(native.Uint32(m.Data[0:4]))
				if error == 0 {
					break done
				}
				return nil, syscall.Errno(-error)
			}
			if m.Header.Type != syscall.RTM_NEWROUTE {
				continue
			}

			var iface *net.Interface = nil
			var ipNet *net.IPNet = nil

			msg := (*RtMsg)(unsafe.Pointer(&m.Data[0:syscall.SizeofRtMsg][0]))

			if msg.Flags&syscall.RTM_F_CLONED != 0 {
				// Ignore cloned routes
				continue
			}

			if msg.Table != syscall.RT_TABLE_MAIN {
				// Ignore non-main tables
				continue
			}

			if msg.Family != syscall.AF_INET {
				// Ignore non-ipv4 routes
				continue
			}

			if msg.Dst_len == 0 {
				// Ignore default routes
				continue
			}

			attrs, err := syscall.ParseNetlinkRouteAttr(&m)
			if err != nil {
				return nil, err
			}
			for _, attr := range attrs {
				switch attr.Attr.Type {
				case syscall.RTA_DST:
					ip := attr.Value
					ipNet = &net.IPNet{
						IP:   ip,
						Mask: net.CIDRMask(int(msg.Dst_len), 8*len(ip)),
					}
				case syscall.RTA_OIF:
					index := int(native.Uint32(attr.Value[0:4]))
					iface, _ = net.InterfaceByIndex(index)
					_ = iface
				}
			}
			if ipNet != nil {
				res = append(res, ipNet)
			}
		}
	}

	return res, nil
}
