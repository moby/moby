package l2disco

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"slices"
	"sync"

	"golang.org/x/sys/unix"
)

// arpData is intended to be per-netns.
type arpData struct {
	mu       sync.Mutex // Lock access to refCount and sd.
	sd       *int       // Socket used to send ARP messages, non-nil only while refCount > 0.
	refCount int        // Count of [UnsolARP] objects using sd.
}

func (ad *arpData) init() (retErr error) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	ad.refCount++
	if ad.sd != nil {
		return nil
	}
	defer func() {
		if retErr != nil {
			ad.refCount = 0
		}
	}()

	sd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("create socket: %w", err)
	}
	ad.sd = &sd

	return nil
}

func (ad *arpData) release() error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if ad.sd == nil || ad.refCount == 0 {
		return fmt.Errorf("invalid release of ARP socket")
	}
	ad.refCount--
	if ad.refCount == 0 {
		err := unix.Close(*ad.sd)
		ad.sd = nil
		return err
	}
	return nil
}

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
	ad  *arpData
	pkt []byte
	sa  *unix.SockaddrLinklayer
}

// NewUnsolARP returns a pointer to an object that can send unsolicited ARPs on
// the interface with ifIndex, for ip and mac.
func (ld *L2Disco) NewUnsolARP(_ context.Context, ip net.IP, mac net.HardwareAddr, ifIndex int) (*UnsolARP, error) {
	if err := ld.arpData.init(); err != nil {
		return nil, err
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
		ad:  &ld.arpData,
		pkt: pkt,
		sa:  sa,
	}, nil
}

// Send sends an unsolicited ARP message.
func (ua *UnsolARP) Send() error {
	return unix.Sendto(*ua.ad.sd, ua.pkt, 0, ua.sa)
}

// Close releases resources.
func (ua *UnsolARP) Close() error {
	return ua.ad.release()
}

// From https://github.com/mdlayher/packet/blob/f9999b41d9cfb0586e75467db1c81cfde4f965ba/packet_linux.go#L238-L248
func htons(i uint16) uint16 {
	var bigEndian [2]byte
	binary.BigEndian.PutUint16(bigEndian[:], i)
	return binary.NativeEndian.Uint16(bigEndian[:])
}
