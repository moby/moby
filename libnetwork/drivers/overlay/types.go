package overlay

// Handy utility types for making unhashable values hashable.

import (
	"net"
	"net/netip"
)

// macAddr is a hashable encoding of a MAC address.
type macAddr uint64

// macAddrOf converts a net.HardwareAddr to a macAddr.
func macAddrOf(mac net.HardwareAddr) macAddr {
	if len(mac) != 6 {
		return 0
	}
	return macAddr(mac[0])<<40 | macAddr(mac[1])<<32 | macAddr(mac[2])<<24 |
		macAddr(mac[3])<<16 | macAddr(mac[4])<<8 | macAddr(mac[5])
}

// HardwareAddr converts a macAddr back to a net.HardwareAddr.
func (p macAddr) HardwareAddr() net.HardwareAddr {
	mac := [6]byte{
		byte(p >> 40), byte(p >> 32), byte(p >> 24),
		byte(p >> 16), byte(p >> 8), byte(p),
	}
	return mac[:]
}

// String returns p.HardwareAddr().String().
func (p macAddr) String() string {
	return p.HardwareAddr().String()
}

// ipmac is a hashable tuple of an IP address and a MAC address suitable for use as a map key.
type ipmac struct {
	ip  netip.Addr
	mac macAddr
}

// ipmacOf is a convenience constructor for creating an ipmac from a [net.HardwareAddr].
func ipmacOf(ip netip.Addr, mac net.HardwareAddr) ipmac {
	return ipmac{
		ip:  ip,
		mac: macAddrOf(mac),
	}
}

func (i ipmac) String() string {
	return i.ip.String() + " " + i.mac.String()
}
