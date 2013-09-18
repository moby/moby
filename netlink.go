package docker

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

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
	len := syscall.SizeofRtMsg
	b := make([]byte, len)
	*(*uint8)(unsafe.Pointer(&b[0:1][0])) = msg.Family
	*(*uint8)(unsafe.Pointer(&b[1:2][0])) = msg.Dst_len
	*(*uint8)(unsafe.Pointer(&b[2:3][0])) = msg.Src_len
	*(*uint8)(unsafe.Pointer(&b[3:4][0])) = msg.Tos
	*(*uint8)(unsafe.Pointer(&b[4:5][0])) = msg.Table
	*(*uint8)(unsafe.Pointer(&b[5:6][0])) = msg.Protocol
	*(*uint8)(unsafe.Pointer(&b[6:7][0])) = msg.Scope
	*(*uint8)(unsafe.Pointer(&b[7:8][0])) = msg.Type
	*(*uint32)(unsafe.Pointer(&b[8:12][0])) = msg.Flags
	return b
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
	len := uint16(syscall.SizeofRtAttr + len(attr.Data))
	b := make([]byte, len)
	*(*uint16)(unsafe.Pointer(&b[0:2][0])) = len
	*(*uint16)(unsafe.Pointer(&b[2:4][0])) = attr.Type
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
	length := rr.Len
	dataBytes := make([][]byte, len(rr.Data))
	for i, data := range rr.Data {
		dataBytes[i] = data.ToWireFormat()
		length = length + uint32(len(dataBytes[i]))
	}
	b := make([]byte, length)
	*(*uint32)(unsafe.Pointer(&b[0:4][0])) = length
	*(*uint16)(unsafe.Pointer(&b[4:6][0])) = rr.Type
	*(*uint16)(unsafe.Pointer(&b[6:8][0])) = rr.Flags
	*(*uint32)(unsafe.Pointer(&b[8:12][0])) = rr.Seq
	*(*uint32)(unsafe.Pointer(&b[12:16][0])) = rr.Pid

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

func newNetlinkRequest(proto, flags, seq int) *NetlinkRequest {
	rr := &NetlinkRequest{}
	rr.Len = uint32(syscall.NLMSG_HDRLEN)
	rr.Type = uint16(proto)
	rr.Flags = syscall.NLM_F_REQUEST | uint16(flags)
	rr.Seq = uint32(seq)
	return rr
}

type NetlinkSocket struct {
	fd int
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

func (s *NetlinkSocket)Close() {
	syscall.Close(s.fd)
}

func (s *NetlinkSocket)Send(request *NetlinkRequest) error {
	if err := syscall.Sendto(s.fd, request.ToWireFormat(), 0, &s.lsa); err != nil {
		return err
	}
	return nil
}

func (s *NetlinkSocket)Recieve() ([]syscall.NetlinkMessage, error) {
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

func (s *NetlinkSocket)GetPid() (uint32, error) {
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

func (s *NetlinkSocket)HandleAck(seq int) error {
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
			if m.Header.Seq != 1 {
				return fmt.Errorf("Wrong Seq nr %d, expected 1", m.Header.Seq)
			}
			if m.Header.Pid != pid {
				return fmt.Errorf("Wrong pid %d, expected %d", m.Header.Pid, pid)
			}
			if m.Header.Type == syscall.NLMSG_DONE {
				break done
			}
			if m.Header.Type == syscall.NLMSG_ERROR {
				error := *(*int32)(unsafe.Pointer(&m.Data[0:4][0]))
				if error == 0 {
					break done
				}
				return syscall.Errno(-error)
			}
		}
	}

	return nil
}



func AddDefaultGw(ip net.IP) error {
	s, err := getNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	family := getIpFamily(ip)

	wb := newNetlinkRequest(syscall.RTM_NEWROUTE, syscall.NLM_F_CREATE|syscall.NLM_F_EXCL|syscall.NLM_F_ACK, 1)

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

	return s.HandleAck(1)

}
