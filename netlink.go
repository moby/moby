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

func AddDefaultGw(ip net.IP) error {
	s, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer syscall.Close(s)
	lsa := &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}
	if err := syscall.Bind(s, lsa); err != nil {
		return err
	}

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

	if err := syscall.Sendto(s, wb.ToWireFormat(), 0, lsa); err != nil {
		return err
	}

done:
	for {
		rb := make([]byte, syscall.Getpagesize())
		nr, _, err := syscall.Recvfrom(s, rb, 0)
		if err != nil {
			return err
		}
		if nr < syscall.NLMSG_HDRLEN {
			return fmt.Errorf("Got short response from netlink")
		}
		rb = rb[:nr]
		msgs, err := syscall.ParseNetlinkMessage(rb)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			lsa, err := syscall.Getsockname(s)
			if err != nil {
				return err
			}
			switch v := lsa.(type) {
			case *syscall.SockaddrNetlink:
				if m.Header.Seq != 1 {
					return fmt.Errorf("Wrong Seq nr %d, expected 1", m.Header.Seq)
				}
				if m.Header.Pid != v.Pid {
					return fmt.Errorf("Wrong pid %d, expected %d", m.Header.Pid, v.Pid)
				}
			default:
				return fmt.Errorf("Wrong socket type")
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

func main() {
	ip := net.ParseIP("172.31.0.1")
	fmt.Println("ip:", len(ip))
	err := AddDefaultGw(ip)
	fmt.Println("res:", err)
}
