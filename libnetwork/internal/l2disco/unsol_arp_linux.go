package l2disco

import (
	"encoding/binary"
	"fmt"
	"net"
	"slices"
	"syscall"
)

var (
	arpTemplate = []byte{
		0x00, 0x01, // Hardware type
		0x08, 0x00, // Protocol
		0x06,       // Hardware address length
		0x04,       // IPv4 address length
		0x00, 0x01, // ARP request
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Sender MAC
		0x00, 0x00, 0x00, 0x00, // Sender IP
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Target MAC (always zeros)
		0x00, 0x00, 0x00, 0x00, // Target IP
	}
	bcastMAC = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
)

type UnsolARP struct {
	pkt []byte
	sd  int
	sa  *syscall.SockaddrLinklayer
}

// To run in container netns.
func NewUnsolARP(ip net.IP, mac net.HardwareAddr, ifIndex int) (*UnsolARP, error) {
	sd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		return nil, fmt.Errorf("create socket: %w", err)
	}

	pkt := slices.Clone(arpTemplate)
	copy(pkt[8:14], mac)
	copy(pkt[14:18], ip)
	copy(pkt[24:28], ip)

	sa := &syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),
		Ifindex:  ifIndex,
		Hatype:   syscall.ARPHRD_ETHER,
		Halen:    uint8(len(bcastMAC)),
	}
	copy(sa.Addr[:], bcastMAC)

	return &UnsolARP{
		pkt: pkt,
		sd:  sd,
		sa:  sa,
	}, nil
}

func (ua *UnsolARP) Send() error {
	return syscall.Sendto(ua.sd, ua.pkt, 0, ua.sa)
}

func (ua *UnsolARP) Close() {
	if ua.sd >= 0 {
		_ = syscall.Close(ua.sd)
		ua.sd = -1
	}
}

// From https://github.com/mdlayher/packet/blob/f9999b41d9cfb0586e75467db1c81cfde4f965ba/packet_linux.go#L238-L248
func htons(i uint16) uint16 {
	var bigEndian [2]byte
	binary.BigEndian.PutUint16(bigEndian[:], i)
	return binary.NativeEndian.Uint16(bigEndian[:])
}
