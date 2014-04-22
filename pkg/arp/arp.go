package arp

import (
	"encoding/binary"
	"log"
	"net"
	"syscall"
	"unsafe"
)

const (
	ARP_HDRLEN  = 28
	ARP_REQUEST = 1
)

func main2() {
	a, err := NewARP("docker0")
	if err != nil {
		log.Fatal(err)
	}
	if err := a.Send("172.17.0.2"); err != nil {
		log.Fatal(err)
	}
	return
}

func NewARP(ifaceName string) (*ARP, error) {
	// Find the index for the given interface
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, err
	}

	// Retrieve the first ip for source
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	srcIP, _, err := net.ParseCIDR(addrs[0].String())
	if err != nil {
		return nil, err
	}

	return &ARP{
		HType:      1,
		PType:      syscall.ETH_P_IP,
		HLen:       6,
		PLen:       4,
		OpCode:     ARP_REQUEST,
		SenderMac:  net.HardwareAddr(iface.HardwareAddr),
		SenderIP:   srcIP.To4(),
		TargetMac:  net.HardwareAddr([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}), // broadcast
		ifaceIndex: iface.Index,
	}, nil
}

type ARP struct {
	HType      uint16
	PType      uint16
	HLen       uint8
	PLen       uint8
	OpCode     uint16
	SenderMac  net.HardwareAddr
	SenderIP   net.IP
	TargetMac  net.HardwareAddr
	TargetIP   net.IP
	ifaceIndex int
}

func (a *ARP) Send(dstIP string) error {
	a.TargetIP = net.ParseIP(dstIP).To4()

	ethernetFrame := make([]byte, syscall.IP_MAXPACKET)
	copy(ethernetFrame[0:6], a.TargetMac)
	copy(ethernetFrame[6:12], a.SenderMac)
	// Ethernet type (ARP)
	ethernetFrame[12] = syscall.ETH_P_ARP / 256
	ethernetFrame[13] = syscall.ETH_P_ARP % 256
	copy(ethernetFrame[14:14+ARP_HDRLEN], a.Bytes())

	// frame length = MAC + MAC + ethernet type + ARP header
	frameLength := 6 + 6 + 2 + ARP_HDRLEN

	sd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, syscall.ETH_P_ALL)
	if err != nil {
		return err
	}
	defer syscall.Close(sd)

	s := syscall.RawSockaddrLinklayer{
		Family:  syscall.AF_PACKET,
		Halen:   6,
		Ifindex: int32(a.ifaceIndex),
	}
	copy(s.Addr[:], a.SenderMac)

	// Didn't find a way to use syscall.Sendto with syscall.RawSockaddrLinklayer, so use direct syscall.
	_, _, e1 := syscall.Syscall6(syscall.SYS_SENDTO, uintptr(sd), uintptr(unsafe.Pointer(&ethernetFrame[0])), uintptr(frameLength), 0, uintptr(unsafe.Pointer(&s)), unsafe.Sizeof(s))
	if e1 != 0 {
		err = e1
		return err
	}
	return nil
}

func (a *ARP) Bytes() []byte {
	buf := make([]byte, ARP_HDRLEN)
	binary.BigEndian.PutUint16(buf[0:2], a.HType)
	binary.BigEndian.PutUint16(buf[2:4], a.PType)
	buf[4] = a.HLen
	buf[5] = a.PLen
	binary.BigEndian.PutUint16(buf[6:8], a.OpCode)
	copy(buf[8:14], a.SenderMac)
	copy(buf[14:18], a.SenderIP)
	copy(buf[18:24], a.TargetMac)
	copy(buf[24:28], a.TargetIP)
	return buf
}
