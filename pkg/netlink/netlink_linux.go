// +build amd64

package netlink

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"syscall"
	"unsafe"
)

const (
	IFNAMSIZ       = 16
	DEFAULT_CHANGE = 0xFFFFFFFF
	IFLA_INFO_KIND = 1
	IFLA_INFO_DATA = 2
	VETH_INFO_PEER = 1
	IFLA_NET_NS_FD = 28
	SIOC_BRADDBR   = 0x89a0
	SIOC_BRADDIF   = 0x89a2
)

var nextSeqNr int

type ifreqHwaddr struct {
	IfrnName   [16]byte
	IfruHwaddr syscall.RawSockaddr
}

type ifreqIndex struct {
	IfrnName  [16]byte
	IfruIndex int32
}

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
	Len() int
	ToWireFormat() []byte
}

type IfInfomsg struct {
	syscall.IfInfomsg
}

func newIfInfomsg(family int) *IfInfomsg {
	return &IfInfomsg{
		IfInfomsg: syscall.IfInfomsg{
			Family: uint8(family),
		},
	}
}

func newIfInfomsgChild(parent *RtAttr, family int) *IfInfomsg {
	msg := newIfInfomsg(family)
	parent.children = append(parent.children, msg)
	return msg
}

func (msg *IfInfomsg) ToWireFormat() []byte {
	native := nativeEndian()

	length := syscall.SizeofIfInfomsg
	b := make([]byte, length)
	b[0] = msg.Family
	b[1] = 0
	native.PutUint16(b[2:4], msg.Type)
	native.PutUint32(b[4:8], uint32(msg.Index))
	native.PutUint32(b[8:12], msg.Flags)
	native.PutUint32(b[12:16], msg.Change)
	return b
}

func (msg *IfInfomsg) Len() int {
	return syscall.SizeofIfInfomsg
}

type IfAddrmsg struct {
	syscall.IfAddrmsg
}

func newIfAddrmsg(family int) *IfAddrmsg {
	return &IfAddrmsg{
		IfAddrmsg: syscall.IfAddrmsg{
			Family: uint8(family),
		},
	}
}

func (msg *IfAddrmsg) ToWireFormat() []byte {
	native := nativeEndian()

	length := syscall.SizeofIfAddrmsg
	b := make([]byte, length)
	b[0] = msg.Family
	b[1] = msg.Prefixlen
	b[2] = msg.Flags
	b[3] = msg.Scope
	native.PutUint32(b[4:8], msg.Index)
	return b
}

func (msg *IfAddrmsg) Len() int {
	return syscall.SizeofIfAddrmsg
}

type RtMsg struct {
	syscall.RtMsg
}

func newRtMsg(family int) *RtMsg {
	return &RtMsg{
		RtMsg: syscall.RtMsg{
			Family:   uint8(family),
			Table:    syscall.RT_TABLE_MAIN,
			Scope:    syscall.RT_SCOPE_UNIVERSE,
			Protocol: syscall.RTPROT_BOOT,
			Type:     syscall.RTN_UNICAST,
		},
	}
}

func (msg *RtMsg) ToWireFormat() []byte {
	native := nativeEndian()

	length := syscall.SizeofRtMsg
	b := make([]byte, length)
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

func (msg *RtMsg) Len() int {
	return syscall.SizeofRtMsg
}

func rtaAlignOf(attrlen int) int {
	return (attrlen + syscall.RTA_ALIGNTO - 1) & ^(syscall.RTA_ALIGNTO - 1)
}

type RtAttr struct {
	syscall.RtAttr
	Data     []byte
	children []NetlinkRequestData
}

func newRtAttr(attrType int, data []byte) *RtAttr {
	return &RtAttr{
		RtAttr: syscall.RtAttr{
			Type: uint16(attrType),
		},
		children: []NetlinkRequestData{},
		Data:     data,
	}
}

func newRtAttrChild(parent *RtAttr, attrType int, data []byte) *RtAttr {
	attr := newRtAttr(attrType, data)
	parent.children = append(parent.children, attr)
	return attr
}

func (a *RtAttr) Len() int {
	l := 0
	for _, child := range a.children {
		l += child.Len() + syscall.SizeofRtAttr
	}
	if l == 0 {
		l++
	}
	return rtaAlignOf(l + len(a.Data))
}

func (a *RtAttr) ToWireFormat() []byte {
	native := nativeEndian()

	length := a.Len()
	buf := make([]byte, rtaAlignOf(length+syscall.SizeofRtAttr))

	if a.Data != nil {
		copy(buf[4:], a.Data)
	} else {
		next := 4
		for _, child := range a.children {
			childBuf := child.ToWireFormat()
			copy(buf[next:], childBuf)
			next += rtaAlignOf(len(childBuf))
		}
	}

	if l := uint16(rtaAlignOf(length)); l != 0 {
		native.PutUint16(buf[0:2], l+1)
	}
	native.PutUint16(buf[2:4], a.Type)

	return buf
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
		length += uint32(len(dataBytes[i]))
	}
	b := make([]byte, length)
	native.PutUint32(b[0:4], length)
	native.PutUint16(b[4:6], rr.Type)
	native.PutUint16(b[6:8], rr.Flags)
	native.PutUint32(b[8:12], rr.Seq)
	native.PutUint32(b[12:16], rr.Pid)

	next := 16
	for _, data := range dataBytes {
		copy(b[next:], data)
		next += len(data)
	}
	return b
}

func (rr *NetlinkRequest) AddData(data NetlinkRequestData) {
	if data != nil {
		rr.Data = append(rr.Data, data)
	}
}

func newNetlinkRequest(proto, flags int) *NetlinkRequest {
	return &NetlinkRequest{
		NlMsghdr: syscall.NlMsghdr{
			Len:   uint32(syscall.NLMSG_HDRLEN),
			Type:  uint16(proto),
			Flags: syscall.NLM_F_REQUEST | uint16(flags),
			Seq:   uint32(getSeq()),
		},
	}
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

func (s *NetlinkSocket) Receive() ([]syscall.NetlinkMessage, error) {
	rb := make([]byte, syscall.Getpagesize())
	nr, _, err := syscall.Recvfrom(s.fd, rb, 0)
	if err != nil {
		return nil, err
	}
	if nr < syscall.NLMSG_HDRLEN {
		return nil, ErrShortResponse
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
	return 0, ErrWrongSockType
}

func (s *NetlinkSocket) HandleAck(seq uint32) error {
	native := nativeEndian()

	pid, err := s.GetPid()
	if err != nil {
		return err
	}

done:
	for {
		msgs, err := s.Receive()
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

func NetworkLinkDown(iface *net.Interface) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_NEWLINK, syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	msg.Change = syscall.IFF_UP
	msg.Flags = 0 & ^syscall.IFF_UP
	msg.Index = int32(iface.Index)
	wb.AddData(msg)

	if err := s.Send(wb); err != nil {
		return err
	}

	return s.HandleAck(wb.Seq)
}

func NetworkSetMTU(iface *net.Interface, mtu int) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_SETLINK, syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	msg.Type = syscall.RTM_SETLINK
	msg.Flags = syscall.NLM_F_REQUEST
	msg.Index = int32(iface.Index)
	msg.Change = DEFAULT_CHANGE
	wb.AddData(msg)

	var (
		b      = make([]byte, 4)
		native = nativeEndian()
	)
	native.PutUint32(b, uint32(mtu))

	data := newRtAttr(syscall.IFLA_MTU, b)
	wb.AddData(data)

	if err := s.Send(wb); err != nil {
		return err
	}
	return s.HandleAck(wb.Seq)
}

// same as ip link set $name master $master
func NetworkSetMaster(iface, master *net.Interface) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_SETLINK, syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	msg.Type = syscall.RTM_SETLINK
	msg.Flags = syscall.NLM_F_REQUEST
	msg.Index = int32(iface.Index)
	msg.Change = DEFAULT_CHANGE
	wb.AddData(msg)

	var (
		b      = make([]byte, 4)
		native = nativeEndian()
	)
	native.PutUint32(b, uint32(master.Index))

	data := newRtAttr(syscall.IFLA_MASTER, b)
	wb.AddData(data)

	if err := s.Send(wb); err != nil {
		return err
	}

	return s.HandleAck(wb.Seq)
}

func NetworkSetNsPid(iface *net.Interface, nspid int) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_SETLINK, syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	msg.Type = syscall.RTM_SETLINK
	msg.Flags = syscall.NLM_F_REQUEST
	msg.Index = int32(iface.Index)
	msg.Change = DEFAULT_CHANGE
	wb.AddData(msg)

	var (
		b      = make([]byte, 4)
		native = nativeEndian()
	)
	native.PutUint32(b, uint32(nspid))

	data := newRtAttr(syscall.IFLA_NET_NS_PID, b)
	wb.AddData(data)

	if err := s.Send(wb); err != nil {
		return err
	}

	return s.HandleAck(wb.Seq)
}

func NetworkSetNsFd(iface *net.Interface, fd int) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_SETLINK, syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	msg.Type = syscall.RTM_SETLINK
	msg.Flags = syscall.NLM_F_REQUEST
	msg.Index = int32(iface.Index)
	msg.Change = DEFAULT_CHANGE
	wb.AddData(msg)

	var (
		b      = make([]byte, 4)
		native = nativeEndian()
	)
	native.PutUint32(b, uint32(fd))

	data := newRtAttr(IFLA_NET_NS_FD, b)
	wb.AddData(data)

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
	return []byte(s + "\000")
}

func nonZeroTerminated(s string) []byte {
	return []byte(s)
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

	if name != "" {
		nameData := newRtAttr(syscall.IFLA_IFNAME, zeroTerminated(name))
		wb.AddData(nameData)
	}

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
func NetworkGetRoutes() ([]Route, error) {
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

	res := make([]Route, 0)

done:
	for {
		msgs, err := s.Receive()
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

			var r Route

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
				// Default routes
				r.Default = true
			}

			attrs, err := syscall.ParseNetlinkRouteAttr(&m)
			if err != nil {
				return nil, err
			}
			for _, attr := range attrs {
				switch attr.Attr.Type {
				case syscall.RTA_DST:
					ip := attr.Value
					r.IPNet = &net.IPNet{
						IP:   ip,
						Mask: net.CIDRMask(int(msg.Dst_len), 8*len(ip)),
					}
				case syscall.RTA_OIF:
					index := int(native.Uint32(attr.Value[0:4]))
					r.Iface, _ = net.InterfaceByIndex(index)
				}
			}
			if r.Default || r.IPNet != nil {
				res = append(res, r)
			}
		}
	}

	return res, nil
}

func getIfSocket() (fd int, err error) {
	for _, socket := range []int{
		syscall.AF_INET,
		syscall.AF_PACKET,
		syscall.AF_INET6,
	} {
		if fd, err = syscall.Socket(socket, syscall.SOCK_DGRAM, 0); err == nil {
			break
		}
	}
	if err == nil {
		return fd, nil
	}
	return -1, err
}

func NetworkChangeName(iface *net.Interface, newName string) error {
	fd, err := getIfSocket()
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	data := [IFNAMSIZ * 2]byte{}
	// the "-1"s here are very important for ensuring we get proper null
	// termination of our new C strings
	copy(data[:IFNAMSIZ-1], iface.Name)
	copy(data[IFNAMSIZ:IFNAMSIZ*2-1], newName)

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCSIFNAME, uintptr(unsafe.Pointer(&data[0]))); errno != 0 {
		return errno
	}
	return nil
}

func NetworkCreateVethPair(name1, name2 string) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	wb := newNetlinkRequest(syscall.RTM_NEWLINK, syscall.NLM_F_CREATE|syscall.NLM_F_EXCL|syscall.NLM_F_ACK)

	msg := newIfInfomsg(syscall.AF_UNSPEC)
	wb.AddData(msg)

	nameData := newRtAttr(syscall.IFLA_IFNAME, zeroTerminated(name1))
	wb.AddData(nameData)

	nest1 := newRtAttr(syscall.IFLA_LINKINFO, nil)
	newRtAttrChild(nest1, IFLA_INFO_KIND, zeroTerminated("veth"))
	nest2 := newRtAttrChild(nest1, IFLA_INFO_DATA, nil)
	nest3 := newRtAttrChild(nest2, VETH_INFO_PEER, nil)

	newIfInfomsgChild(nest3, syscall.AF_UNSPEC)
	newRtAttrChild(nest3, syscall.IFLA_IFNAME, zeroTerminated(name2))

	wb.AddData(nest1)

	if err := s.Send(wb); err != nil {
		return err
	}
	return s.HandleAck(wb.Seq)
}

// Create the actual bridge device.  This is more backward-compatible than
// netlink.NetworkLinkAdd and works on RHEL 6.
func CreateBridge(name string, setMacAddr bool) error {
	s, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, syscall.IPPROTO_IP)
	if err != nil {
		// ipv6 issue, creating with ipv4
		s, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_IP)
		if err != nil {
			return err
		}
	}
	defer syscall.Close(s)

	nameBytePtr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(s), SIOC_BRADDBR, uintptr(unsafe.Pointer(nameBytePtr))); err != 0 {
		return err
	}
	if setMacAddr {
		return setBridgeMacAddress(s, name)
	}
	return nil
}

// Add a slave to abridge device.  This is more backward-compatible than
// netlink.NetworkSetMaster and works on RHEL 6.
func AddToBridge(iface, master *net.Interface) error {
	s, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, syscall.IPPROTO_IP)
	if err != nil {
		// ipv6 issue, creating with ipv4
		s, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_IP)
		if err != nil {
			return err
		}
	}
	defer syscall.Close(s)

	ifr := ifreqIndex{}
	copy(ifr.IfrnName[:], master.Name)
	ifr.IfruIndex = int32(iface.Index)

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(s), SIOC_BRADDIF, uintptr(unsafe.Pointer(&ifr))); err != 0 {
		return err
	}

	return nil
}

func setBridgeMacAddress(s int, name string) error {
	ifr := ifreqHwaddr{}
	ifr.IfruHwaddr.Family = syscall.ARPHRD_ETHER
	copy(ifr.IfrnName[:], name)

	for i := 0; i < 6; i++ {
		ifr.IfruHwaddr.Data[i] = int8(rand.Intn(255))
	}

	ifr.IfruHwaddr.Data[0] &^= 0x1 // clear multicast bit
	ifr.IfruHwaddr.Data[0] |= 0x2  // set local assignment bit (IEEE802)

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(s), syscall.SIOCSIFHWADDR, uintptr(unsafe.Pointer(&ifr))); err != 0 {
		return err
	}
	return nil
}
