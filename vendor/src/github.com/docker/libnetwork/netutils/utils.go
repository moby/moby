// Network utility functions.

package netutils

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
)

var (
	// ErrNetworkOverlapsWithNameservers preformatted error
	ErrNetworkOverlapsWithNameservers = errors.New("requested network overlaps with nameserver")
	// ErrNetworkOverlaps preformatted error
	ErrNetworkOverlaps = errors.New("requested network overlaps with existing network")
	// ErrNoDefaultRoute preformatted error
	ErrNoDefaultRoute = errors.New("no default route")

	networkGetRoutesFct = netlink.RouteList
)

// ErrInvalidProtocolBinding is returned when the port binding protocol is not valid.
type ErrInvalidProtocolBinding string

func (ipb ErrInvalidProtocolBinding) Error() string {
	return fmt.Sprintf("invalid transport protocol: %s", string(ipb))
}

// TransportPort represent a local Layer 4 endpoint
type TransportPort struct {
	Proto Protocol
	Port  uint16
}

// GetCopy returns a copy of this TransportPort structure instance
func (t *TransportPort) GetCopy() TransportPort {
	return TransportPort{Proto: t.Proto, Port: t.Port}
}

// PortBinding represent a port binding between the container an the host
type PortBinding struct {
	Proto    Protocol
	IP       net.IP
	Port     uint16
	HostIP   net.IP
	HostPort uint16
}

// HostAddr returns the host side transport address
func (p PortBinding) HostAddr() (net.Addr, error) {
	switch p.Proto {
	case UDP:
		return &net.UDPAddr{IP: p.HostIP, Port: int(p.HostPort)}, nil
	case TCP:
		return &net.TCPAddr{IP: p.HostIP, Port: int(p.HostPort)}, nil
	default:
		return nil, ErrInvalidProtocolBinding(p.Proto.String())
	}
}

// ContainerAddr returns the container side transport address
func (p PortBinding) ContainerAddr() (net.Addr, error) {
	switch p.Proto {
	case UDP:
		return &net.UDPAddr{IP: p.IP, Port: int(p.Port)}, nil
	case TCP:
		return &net.TCPAddr{IP: p.IP, Port: int(p.Port)}, nil
	default:
		return nil, ErrInvalidProtocolBinding(p.Proto.String())
	}
}

// GetCopy returns a copy of this PortBinding structure instance
func (p *PortBinding) GetCopy() PortBinding {
	return PortBinding{
		Proto:    p.Proto,
		IP:       GetIPCopy(p.IP),
		Port:     p.Port,
		HostIP:   GetIPCopy(p.HostIP),
		HostPort: p.HostPort,
	}
}

// Equal checks if this instance of PortBinding is equal to the passed one
func (p *PortBinding) Equal(o *PortBinding) bool {
	if p == o {
		return true
	}

	if o == nil {
		return false
	}

	if p.Proto != o.Proto || p.Port != o.Port || p.HostPort != o.HostPort {
		return false
	}

	if p.IP != nil {
		if !p.IP.Equal(o.IP) {
			return false
		}
	} else {
		if o.IP != nil {
			return false
		}
	}

	if p.HostIP != nil {
		if !p.HostIP.Equal(o.HostIP) {
			return false
		}
	} else {
		if o.HostIP != nil {
			return false
		}
	}

	return true
}

const (
	// ICMP is for the ICMP ip protocol
	ICMP = 1
	// TCP is for the TCP ip protocol
	TCP = 6
	// UDP is for the UDP ip protocol
	UDP = 17
)

// Protocol represents a IP protocol number
type Protocol uint8

func (p Protocol) String() string {
	switch p {
	case ICMP:
		return "icmp"
	case TCP:
		return "tcp"
	case UDP:
		return "udp"
	default:
		return fmt.Sprintf("%d", p)
	}
}

// ParseProtocol returns the respective Protocol type for the passed string
func ParseProtocol(s string) Protocol {
	switch strings.ToLower(s) {
	case "icmp":
		return ICMP
	case "udp":
		return UDP
	case "tcp":
		return TCP
	default:
		return 0
	}
}

// CheckNameserverOverlaps checks whether the passed network overlaps with any of the nameservers
func CheckNameserverOverlaps(nameservers []string, toCheck *net.IPNet) error {
	if len(nameservers) > 0 {
		for _, ns := range nameservers {
			_, nsNetwork, err := net.ParseCIDR(ns)
			if err != nil {
				return err
			}
			if NetworkOverlaps(toCheck, nsNetwork) {
				return ErrNetworkOverlapsWithNameservers
			}
		}
	}
	return nil
}

// CheckRouteOverlaps checks whether the passed network overlaps with any existing routes
func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := networkGetRoutesFct(nil, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Dst != nil && NetworkOverlaps(toCheck, network.Dst) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// NetworkOverlaps detects overlap between one IPNet and another
func NetworkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	// Check if both netX and netY are ipv4 or ipv6
	if (netX.IP.To4() != nil && netY.IP.To4() != nil) ||
		(netX.IP.To4() == nil && netY.IP.To4() == nil) {
		if firstIP, _ := NetworkRange(netX); netY.Contains(firstIP) {
			return true
		}
		if firstIP, _ := NetworkRange(netY); netX.Contains(firstIP) {
			return true
		}
	}
	return false
}

// NetworkRange calculates the first and last IP addresses in an IPNet
func NetworkRange(network *net.IPNet) (net.IP, net.IP) {
	var netIP net.IP
	if network.IP.To4() != nil {
		netIP = network.IP.To4()
	} else if network.IP.To16() != nil {
		netIP = network.IP.To16()
	} else {
		return nil, nil
	}

	lastIP := make([]byte, len(netIP), len(netIP))
	for i := 0; i < len(netIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return netIP.Mask(network.Mask), net.IP(lastIP)
}

// GetIfaceAddr returns the first IPv4 address and slice of IPv6 addresses for the specified network interface
func GetIfaceAddr(name string) (net.Addr, []net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil, err
	}
	var addrs4 []net.Addr
	var addrs6 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); ip4 != nil {
			addrs4 = append(addrs4, addr)
		} else if ip6 := ip.To16(); len(ip6) == net.IPv6len {
			addrs6 = append(addrs6, addr)
		}
	}
	switch {
	case len(addrs4) == 0:
		return nil, nil, fmt.Errorf("Interface %v has no IPv4 addresses", name)
	case len(addrs4) > 1:
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], addrs6, nil
}

// GenerateRandomMAC returns a new 6-byte(48-bit) hardware address (MAC)
func GenerateRandomMAC() net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)
	// The first byte of the MAC address has to comply with these rules:
	// 1. Unicast: Set the least-significant bit to 0.
	// 2. Address is locally administered: Set the second-least-significant bit (U/L) to 1.
	// 3. As "small" as possible: The veth address has to be "smaller" than the bridge address.
	hw[0] = 0x02
	// The first 24 bits of the MAC represent the Organizationally Unique Identifier (OUI).
	// Since this address is locally administered, we can do whatever we want as long as
	// it doesn't conflict with other addresses.
	hw[1] = 0x42
	// Randomly generate the remaining 4 bytes (2^32)
	_, err := rand.Read(hw[2:])
	if err != nil {
		return nil
	}
	return hw
}

// GenerateRandomName returns a new name joined with a prefix.  This size
// specified is used to truncate the randomly generated value
func GenerateRandomName(prefix string, size int) (string, error) {
	id := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(id)[:size], nil
}

// GetMacCopy returns a copy of the passed MAC address
func GetMacCopy(from net.HardwareAddr) net.HardwareAddr {
	to := make(net.HardwareAddr, len(from))
	copy(to, from)
	return to
}

// GetIPCopy returns a copy of the passed IP address
func GetIPCopy(from net.IP) net.IP {
	to := make(net.IP, len(from))
	copy(to, from)
	return to
}

// GetIPNetCopy returns a copy of the passed IP Network
func GetIPNetCopy(from *net.IPNet) *net.IPNet {
	if from == nil {
		return nil
	}
	bm := make(net.IPMask, len(from.Mask))
	copy(bm, from.Mask)
	return &net.IPNet{IP: GetIPCopy(from.IP), Mask: bm}
}

// CompareIPNet returns equal if the two IP Networks are equal
func CompareIPNet(a, b *net.IPNet) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.IP.Equal(b.IP) && bytes.Equal(a.Mask, b.Mask)
}
