package l2disco

import (
	"fmt"
	"net"
	"slices"

	"golang.org/x/sys/unix"
)

var naTemplate = []byte{
	0x88,       // Type (136=NA)
	0x00,       // Code (always 0)
	0x00, 0x00, // Checksum (filled in by the kernel)
	0x20,             // Flags, Router=0, Solicited=0, Override=1
	0x00, 0x00, 0x00, // Reserved
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Target IP
	0x02,                               // Option - target link layer address
	0x01,                               // Option length (32)
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Target MAC
}

type UnsolNA struct {
	pkt []byte
	sd  int
	sa  *unix.SockaddrInet6
}

// NewUnsolNA returns a pointer to an object that can send unsolicited Neighbour
// Advertisements on the interface with ifIndex, for ip and mac.
// https://datatracker.ietf.org/doc/html/rfc4861#section-4.4
func NewUnsolNA(ip net.IP, mac net.HardwareAddr, ifIndex int) (*UnsolNA, error) {
	sd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_ICMPV6)
	if err != nil {
		return nil, fmt.Errorf("create socket: %w", err)
	}

	// https://datatracker.ietf.org/doc/html/rfc4861#section-3.1
	//  By setting the Hop Limit to 255, Neighbor Discovery is immune to
	//  off-link senders that accidentally or intentionally send ND
	//  messages.
	if err := unix.SetsockoptInt(sd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS, 255); err != nil {
		_ = unix.Close(sd)
		return nil, fmt.Errorf("set hop limit: %w", err)
	}

	saBind := &unix.SockaddrInet6{}
	copy(saBind.Addr[:], ip)
	if err := unix.Bind(sd, saBind); err != nil {
		_ = unix.Close(sd)
		return nil, fmt.Errorf("bind socket: %w", err)
	}

	pkt := slices.Clone(naTemplate)
	copy(pkt[8:24], ip)
	copy(pkt[26:32], mac)

	sa := &unix.SockaddrInet6{}
	copy(sa.Addr[:], net.IPv6linklocalallnodes)

	return &UnsolNA{
		pkt: pkt,
		sd:  sd,
		sa:  sa,
	}, nil
}

// Send sends an unsolicited ARP message.
func (un *UnsolNA) Send() error {
	return unix.Sendto(un.sd, un.pkt, 0, un.sa)
}

// Close releases resources.
func (un *UnsolNA) Close() {
	if un.sd >= 0 {
		_ = unix.Close(un.sd)
		un.sd = -1
	}
}
