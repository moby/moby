package overlay

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/net/bpf"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

func TestVNIMatchBPF(t *testing.T) {
	// The BPF filter program under test uses Linux extensions which are not
	// emulated by any user-space BPF interpreters. It is also classic BPF,
	// which cannot be tested in-kernel using the bpf(BPF_PROG_RUN) syscall.
	// The best we can do without actually programming it into an iptables
	// rule and end-to-end testing it is to attach it as a socket filter to
	// a raw socket and test which loopback packets make it through.
	//
	// Modern kernels transpile cBPF programs into eBPF for execution, so a
	// possible future direction would be to extract the transpiler and
	// convert the program under test to eBPF so it could be loaded and run
	// using the bpf(2) syscall.
	// https://elixir.bootlin.com/linux/v6.2/source/net/core/filter.c#L559
	// Though the effort would be better spent on adding nftables support to
	// libnetwork so this whole BPF program could be replaced with a native
	// nftables '@th' match expression.
	//
	// The filter could be manually e2e-tested for both IPv4 and IPv6 by
	// programming ip[6]tables rules which log matching packets and sending
	// test packets loopback using netcat. All the necessary information
	// (bytecode and an acceptable test vector) is logged by this test.
	//
	//     $ sudo ip6tables -A INPUT -p udp -s ::1 -d ::1 -m bpf \
	//         --bytecode "${bpf_program_under_test}" \
	//         -j LOG --log-prefix '[IPv6 VNI match]:'
	//     $ <<<"${udp_payload_hexdump}" xxd -r -p | nc -u -6 localhost 30000
	//     $ sudo dmesg

	loopback := net.IPv4(127, 0, 0, 1)

	// Reserve an ephemeral UDP port for loopback testing.
	// Binding to a TUN device would be more hermetic, but is much more effort to set up.
	reservation, err := net.ListenUDP("udp", &net.UDPAddr{IP: loopback, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer reservation.Close()
	daddr := reservation.LocalAddr().(*net.UDPAddr).AddrPort()

	sender, err := net.DialUDP("udp", nil, reservation.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	saddr := sender.LocalAddr().(*net.UDPAddr).AddrPort()

	// There doesn't seem to be a way to receive the entire Layer-3 IPv6
	// packet including the fixed IP header using the portable raw sockets
	// API. That can only be done from an AF_PACKET socket, and it is
	// unclear whether 'ld poff' would behave the same in a BPF program
	// attached to such a socket as in an xt_bpf match.
	c, err := net.ListenIP("ip4:udp", &net.IPAddr{IP: loopback})
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skip("test requires CAP_NET_RAW")
		}
		t.Fatal(err)
	}
	defer c.Close()

	pc := ipv4.NewPacketConn(c)

	testvectors := []uint32{
		0,
		1,
		0x08,
		42,
		0x80,
		0xfe,
		0xff,
		0x100,
		0xfff,  // 4095
		0x1000, // 4096
		0x1001,
		0x10000,
		0xfffffe,
		0xffffff, // Max VNI
	}
	for _, vni := range []uint32{1, 42, 0x100, 0x1000, 0xfffffe, 0xffffff} {
		t.Run(fmt.Sprintf("vni=%d", vni), func(t *testing.T) {
			setBPF(t, pc, vniMatchBPF(vni))

			for _, v := range testvectors {
				pkt := appendVXLANHeader(nil, v)
				pkt = append(pkt, []byte{0xde, 0xad, 0xbe, 0xef}...)
				if _, err := sender.Write(pkt); err != nil {
					t.Fatal(err)
				}

				rpkt, ok := readUDPPacketFromRawSocket(t, pc, saddr, daddr)
				// Sanity check: the only packets readUDPPacketFromRawSocket
				// should return are ones we sent.
				if ok && !bytes.Equal(pkt, rpkt) {
					t.Fatalf("received unexpected packet: % x", rpkt)
				}
				if ok != (v == vni) {
					t.Errorf("unexpected packet tagged with vni=%d (got %v, want %v)", v, ok, v == vni)
				}
			}
		})
	}
}

func appendVXLANHeader(b []byte, vni uint32) []byte {
	// https://tools.ietf.org/html/rfc7348#section-5
	b = append(b, []byte{0x08, 0x00, 0x00, 0x00}...)
	return binary.BigEndian.AppendUint32(b, vni<<8)
}

func setBPF(t *testing.T, c *ipv4.PacketConn, fprog []bpf.RawInstruction) {
	// https://natanyellin.com/posts/ebpf-filtering-done-right/
	blockall, _ := bpf.Assemble([]bpf.Instruction{bpf.RetConstant{Val: 0}})
	if err := c.SetBPF(blockall); err != nil {
		t.Fatal(err)
	}
	ms := make([]ipv4.Message, 100)
	for {
		n, err := c.ReadBatch(ms, unix.MSG_DONTWAIT)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				break
			}
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}

	t.Logf("setting socket filter: %v", marshalXTBPF(fprog))
	if err := c.SetBPF(fprog); err != nil {
		t.Fatal(err)
	}
}

// readUDPPacketFromRawSocket reads raw IP packets from pc until a UDP packet
// which matches the (src, dst) 4-tuple is found or the receive buffer is empty,
// and returns the payload of the UDP packet.
func readUDPPacketFromRawSocket(t *testing.T, pc *ipv4.PacketConn, src, dst netip.AddrPort) ([]byte, bool) {
	t.Helper()

	ms := []ipv4.Message{
		{Buffers: [][]byte{make([]byte, 1500)}},
	}

	// Set a time limit to prevent an infinite loop if there is a lot of
	// loopback traffic being captured which prevents the buffer from
	// emptying.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		n, err := pc.ReadBatch(ms, unix.MSG_DONTWAIT)
		if err != nil {
			if !errors.Is(err, unix.EAGAIN) {
				t.Fatal(err)
			}
			break
		}
		if n == 0 {
			break
		}
		pkt := ms[0].Buffers[0][:ms[0].N]
		psrc, pdst, payload, ok := parseUDP(pkt)
		// Discard captured packets which belong to other unrelated flows.
		if !ok || psrc != src || pdst != dst {
			t.Logf("discarding packet:\n% x", pkt)
			continue
		}
		t.Logf("received packet (%v -> %v):\n% x", psrc, pdst, payload)
		// While not strictly required, copy payload into a new
		// slice which does not share a backing array with pkt
		// so the IP and UDP headers can be garbage collected.
		return append([]byte(nil), payload...), true
	}
	return nil, false
}

func parseIPv4(b []byte) (src, dst netip.Addr, protocol byte, payload []byte, ok bool) {
	if len(b) < 20 {
		return netip.Addr{}, netip.Addr{}, 0, nil, false
	}
	hlen := int(b[0]&0x0f) * 4
	if hlen < 20 {
		return netip.Addr{}, netip.Addr{}, 0, nil, false
	}
	src, _ = netip.AddrFromSlice(b[12:16])
	dst, _ = netip.AddrFromSlice(b[16:20])
	protocol = b[9]
	payload = b[hlen:]
	return src, dst, protocol, payload, true
}

// parseUDP parses the IP and UDP headers from the raw Layer-3 packet data in b.
func parseUDP(b []byte) (src, dst netip.AddrPort, payload []byte, ok bool) {
	srcip, dstip, protocol, ippayload, ok := parseIPv4(b)
	if !ok {
		return netip.AddrPort{}, netip.AddrPort{}, nil, false
	}
	if protocol != 17 {
		return netip.AddrPort{}, netip.AddrPort{}, nil, false
	}
	if len(ippayload) < 8 {
		return netip.AddrPort{}, netip.AddrPort{}, nil, false
	}
	sport := binary.BigEndian.Uint16(ippayload[0:2])
	dport := binary.BigEndian.Uint16(ippayload[2:4])
	src = netip.AddrPortFrom(srcip, sport)
	dst = netip.AddrPortFrom(dstip, dport)
	payload = ippayload[8:]
	return src, dst, payload, true
}
