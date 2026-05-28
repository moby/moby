package l2disco

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"slices"

	"golang.org/x/sys/unix"
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
	sa  *unix.SockaddrLinklayer
}

// NewUnsolARP returns a pointer to an object that can send unsolicited ARPs on
// the interface with ifIndex, for ip and mac.
func NewUnsolARP(_ context.Context, ip net.IP, mac net.HardwareAddr, ifIndex int) (*UnsolARP, error) {
	sd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("create socket: %w", err)
	}

	pkt := slices.Clone(arpTemplate)
	copy(pkt[8:14], mac)
	copy(pkt[14:18], ip)
	copy(pkt[24:28], ip)

	sa := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ARP),
		Ifindex:  ifIndex,
		Hatype:   unix.ARPHRD_ETHER,
		Halen:    uint8(len(bcastMAC)),
	}
	copy(sa.Addr[:], bcastMAC)

	return &UnsolARP{
		pkt: pkt,
		sd:  sd,
		sa:  sa,
	}, nil
}

// Send sends an unsolicited ARP message.
func (ua *UnsolARP) Send() error {
	return unix.Sendto(ua.sd, ua.pkt, 0, ua.sa)
}

// Close releases resources.
func (ua *UnsolARP) Close() error {
	if ua.sd >= 0 {
		err := unix.Close(ua.sd)
		ua.sd = -1
		return err
	}
	return nil
}

// From https://github.com/mdlayher/packet/blob/f9999b41d9cfb0586e75467db1c81cfde4f965ba/packet_linux.go#L238-L248
func htons(i uint16) uint16 {
	var bigEndian [2]byte
	binary.BigEndian.PutUint16(bigEndian[:], i)
	return binary.NativeEndian.Uint16(bigEndian[:])
}
