package network

// Collect and decode broadcast ARP and unsolicited ICMP6 Neighbour Advertisement messages...

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

// TimestampedPkt has a Data slice representing a packet, ReceivedAt is a timestamp
// set after the packet was received in user-space.
type TimestampedPkt struct {
	ReceivedAt time.Time
	Data       []byte
}

// CollectBcastARPs collects broadcast ARPs from interface ifname.
// It returns a stop function, to stop collection and return a slice of collected packets (with
// timestamps added when they were received in userspace).
func CollectBcastARPs(t *testing.T, ifname string) (stop func() []TimestampedPkt) {
	sd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, int(htons(unix.ETH_P_ARP)))
	assert.NilError(t, err)
	assert.Assert(t, sd >= 0)

	link, err := nlwrap.LinkByName(ifname)
	assert.NilError(t, err)

	err = unix.Bind(sd, &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ARP),
		Pkttype:  unix.PACKET_BROADCAST,
		Ifindex:  link.Attrs().Index,
	})
	assert.NilError(t, err)

	return collectPackets(t, sd, "ARP")
}

// From https://github.com/mdlayher/packet/blob/f9999b41d9cfb0586e75467db1c81cfde4f965ba/packet_linux.go#L238-L248
func htons(i uint16) uint16 {
	var bigEndian [2]byte
	binary.BigEndian.PutUint16(bigEndian[:], i)
	return binary.NativeEndian.Uint16(bigEndian[:])
}

// CollectICMP6 collects ICMP6 packets sent to the all nodes address.
// It returns a stop function, to stop collection and return a slice of collected packets (with
// timestamps added when they were received in userspace).
func CollectICMP6(t *testing.T, ifname string) (stop func() []TimestampedPkt) {
	sd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_ICMPV6)
	assert.NilError(t, err)
	assert.Assert(t, sd >= 0)

	link, err := nlwrap.LinkByName(ifname)
	assert.NilError(t, err)

	mreq := &unix.IPv6Mreq{
		Interface: uint32(link.Attrs().Index),
	}
	copy(mreq.Multiaddr[:], net.IPv6linklocalallnodes)
	err = unix.SetsockoptIPv6Mreq(sd, unix.IPPROTO_IPV6, unix.IPV6_JOIN_GROUP, mreq)
	assert.NilError(t, err)

	return collectPackets(t, sd, "ICMP6")
}

func collectPackets(t *testing.T, sd int, what string) (stop func() []TimestampedPkt) {
	err := unix.SetsockoptTimeval(sd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 1})
	assert.NilError(t, err)

	stopC := make(chan struct{})
	stoppedC := make(chan struct{})
	var pkts []TimestampedPkt
	go func() {
		defer close(stoppedC)
		defer unix.Close(sd)
		for {
			buf := make([]byte, 50)
			// Blocking read, if no packet is received it'll return an EWOULDBLOCK after a timeout.
			n, err := unix.Read(sd, buf)
			// If the stop function has been called, return. That closes stoppedC to confirm that
			// nothing else will be added to pkts. (Read() probably hit its timeout but, even
			// if it read a packet, it was racing the stop and the packet can just be dropped.)
			select {
			case <-stopC:
				return
			default:
			}
			if err != nil {
				if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EWOULDBLOCK) {
					continue
				}
				t.Log(what, "read error:", err, "sd", sd)
				return
			}
			pkts = append(pkts, TimestampedPkt{
				ReceivedAt: time.Now(),
				Data:       buf[:n],
			})
		}
	}()

	// Return a stop function - packet collection will continue until this is called.
	return func() []TimestampedPkt {
		select {
		case <-stopC:
		default:
			close(stopC)
		}
		// Wait for confirmation that packet collection has stopped, to be sure the
		// pkts slice won't change after the return.
		<-stoppedC
		return pkts
	}
}

// UnpackUnsolARP checks the packet is a valid Ethernet unsolicited/broadcast ARP
// request packet. It returns sender hardware and protocol addresses,
// and true if it is - else, an error.
func UnpackUnsolARP(pkt TimestampedPkt) (sh net.HardwareAddr, sp netip.Addr, err error) {
	if len(pkt.Data) != 28 {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("packet size %d", len(pkt.Data))
	}
	// Hardware type (1)
	if pkt.Data[0] != 0 || pkt.Data[1] != 1 {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("hardware type %v", pkt.Data[0:2])
	}
	// Protocol type (0x800).
	if pkt.Data[2] != 8 || pkt.Data[3] != 0 {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("protocol type %v", pkt.Data[2:4])
	}
	// Hardware length (6)
	if pkt.Data[4] != 6 {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("hardware length %v", pkt.Data[4])
	}
	// Protocol length (4)
	if pkt.Data[5] != 4 {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("protocol length %v", pkt.Data[5])
	}
	// Operation (1=request, 2=reply)
	if pkt.Data[6] != 0 || pkt.Data[7] != 1 {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("operation %v", pkt.Data[6:8])
	}

	// Sender hardware address
	sh = make(net.HardwareAddr, 6)
	copy(sh, pkt.Data[8:14])
	// Sender protocol address
	sp, _ = netip.AddrFromSlice(pkt.Data[14:18])

	// Target hardware address
	if slices.Compare(pkt.Data[18:24], net.HardwareAddr{0, 0, 0, 0, 0, 0}) != 0 {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("nonzero target mac address %s", hex.EncodeToString(pkt.Data[18:24]))
	}

	// Target protocol address
	if tp, _ := netip.AddrFromSlice(pkt.Data[24:28]); tp != sp {
		return net.HardwareAddr{}, netip.Addr{}, fmt.Errorf("sender and target IP addresses differ, %s/%s", sp, tp)
	}

	return sh, sp, nil
}

// UnpackUnsolNA returns the hardware (MAC) and protocol (IP) addresses from the
// packet, if it is an unsolicited Neighbour Advertisement message with a
// link address option. Otherwise, it returns an error.
func UnpackUnsolNA(pkt TimestampedPkt) (th net.HardwareAddr, tp netip.Addr, err error) {
	// Treat the packet as invalid unless it's sized for a NA message
	// with a link address option.
	if len(pkt.Data) != 32 {
		return th, tp, fmt.Errorf("packet size %d", len(pkt.Data))
	}
	// Type (136=NA)
	if pkt.Data[0] != 136 {
		return th, tp, fmt.Errorf("type %d", pkt.Data[0])
	}
	// Code
	if pkt.Data[1] != 0 {
		return th, tp, fmt.Errorf("code %d", pkt.Data[1])
	}
	// Checksum pkt.Data[2:4].
	// - calculated by the kernel on packets sent by dockerd, not checking that calc here.
	// Router flag (not sent by a router)
	if pkt.Data[4]&0x80 != 0 {
		return th, tp, errors.New("flag Router is set")
	}
	// Solicited flag (unsolicited)
	if pkt.Data[4]&0x40 != 0 {
		return th, tp, errors.New("flag Solicited is set")
	}
	// Override flag (SHOULD be set in an unsolicited advertisement)
	if pkt.Data[4]&0x20 == 0 {
		return th, tp, errors.New("flag Override is not set")
	}
	// Reserved pkt.Data[4:8]
	// Target address
	tp, _ = netip.AddrFromSlice(pkt.Data[8:24])
	// Options (02=link address, 01=length 8)
	if pkt.Data[24] != 2 || pkt.Data[25] != 1 {
		return th, tp, fmt.Errorf("option %d length %d", pkt.Data[24], pkt.Data[25])
	}
	// Link address
	th = make(net.HardwareAddr, 6)
	copy(th, pkt.Data[26:32])

	return th, tp, nil
}
