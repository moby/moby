// Package types contains types that are common across libnetwork project
package types

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

// UUID represents a globally unique ID of various resources like network and endpoint
type UUID string

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
