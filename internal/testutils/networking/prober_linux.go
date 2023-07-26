package networking

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/mdlayher/packet"
	"golang.org/x/sys/unix"
)

type SynProber struct {
	Iface   string
	SrcMAC  net.HardwareAddr
	DstMAC  net.HardwareAddr
	SrcIP   netip.Addr
	DstIP   netip.Addr
	SrcPort uint16
	DstPort uint16
}

var ErrNoSynAck = errors.New("no SYN-ACK received")

// Probe sends a SYN packet to the source MAC set its SynProber receiver and then checks if any SYN-ACK is sent back by
// that source. It returns both the SYN packet sent and an error if no SYN-ACK was received before reaching the
// deadline.
func (p SynProber) Probe(deadline time.Time) ([]byte, error) {
	iface, err := net.InterfaceByName(p.Iface)
	if err != nil {
		return nil, fmt.Errorf("could not get interface %s: %w", p.Iface, err)
	}

	l, err := packet.Listen(iface, packet.Raw, syscall.ETH_P_IP, nil)
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			return nil, errors.New("you need CAP_NET_RAW")
		}
		return nil, err
	}
	defer l.Close()

	eth := layers.Ethernet{
		SrcMAC:       p.SrcMAC,
		DstMAC:       p.DstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := layers.IPv4{
		Version:  4,
		IHL:      5,
		Id:       1,
		TTL:      60,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    net.IP(p.SrcIP.AsSlice()),
		DstIP:    net.IP(p.DstIP.AsSlice()),
	}
	tcp := layers.TCP{
		SrcPort: layers.TCPPort(p.SrcPort),
		DstPort: layers.TCPPort(p.DstPort),
		SYN:     true,
		Window:  8192,
	}
	tcp.SetNetworkLayerForChecksum(&ip)

	serializeBuf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	err = gopacket.SerializeLayers(serializeBuf, opts, &eth, &ip, &tcp)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize crafted packet: %w", err)
	}

	// As can be seen in [gopacket source code], Ethernet frames are padded with extra bytes to comply to the minimum
	// frame length of 60 bytes (without FCS) defined by IEEE 802.3. However, the kernel will strip those extra bytes
	// when receiving this frame.
	// As the layers serialization above produces a 54-bytes long Ethernet frame, gopacket adds 6 padding bytes we
	// strip here to make sure what's returned by this function is exactly what an AF_RAW socket will see on its
	// receiving end.
	// [gopacket source code]: https://github.com/google/gopacket/blob/32ee38206866f44a74a6033ec26aeeb474506804/layers/ethernet.go#L95-L102
	buf := serializeBuf.Bytes()[:54]

	daddr := &packet.Addr{HardwareAddr: p.DstMAC}
	fmt.Printf("Sending Ethernet frame to %s (%d bytes).\n%s\n", daddr.String(), len(buf), hex.Dump(buf))
	if _, err := l.WriteTo(buf, daddr); err != nil {
		return nil, err
	}

	l.SetReadDeadline(deadline)
	for {
		buf := make([]byte, 1500)
		n, _, err := l.ReadFrom(buf)
		if err != nil {
			if os.IsTimeout(err) {
				break
			}
			return nil, err
		}

		gopkt := gopacket.NewPacket(buf[:n], layers.LayerTypeEthernet, gopacket.DecodeOptions{
			NoCopy: true,
		})

		ansIPLayer := gopkt.Layer(layers.LayerTypeIPv4)
		if ansIPLayer == nil {
			continue
		}

		ansIP := ansIPLayer.(*layers.IPv4)
		if !ansIP.SrcIP.Equal(ip.DstIP) || !ansIP.DstIP.Equal(ip.SrcIP) {
			continue
		}

		ansTCPLayer := gopkt.Layer(layers.LayerTypeTCP)
		if ansTCPLayer == nil {
			continue
		}

		ansTCP := ansTCPLayer.(*layers.TCP)
		if ansTCP.SrcPort != tcp.DstPort || ansTCP.DstPort != tcp.SrcPort {
			continue
		}

		if ansTCP.ACK {
			fmt.Printf("Received a SYN-ACK from %s:%d!\n", ansIP.SrcIP, ansTCP.SrcPort)
			return buf, nil
		}
	}

	return buf, ErrNoSynAck
}
