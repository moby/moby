// Package hashable provides handy utility types for making unhashable values
// hashable.
package hashable

import (
	"net"
	"net/netip"
)

// MACAddr is a hashable encoding of a MAC address.
type MACAddr uint64

// MACAddrFromSlice parses the 6-byte slice as a MAC-48 address.
// Note that a [net.HardwareAddr] can be passed directly as the []byte argument.
// If slice's length is not 6, MACAddrFromSlice returns 0, false.
func MACAddrFromSlice(slice net.HardwareAddr) (MACAddr, bool) {
	if len(slice) != 6 {
		return 0, false
	}
	return MACAddrFrom6([6]byte(slice)), true
}

// MACAddrFrom6 returns the address of the MAC-48 address
// given by the bytes in addr.
func MACAddrFrom6(addr [6]byte) MACAddr {
	return MACAddr(addr[0])<<40 | MACAddr(addr[1])<<32 | MACAddr(addr[2])<<24 |
		MACAddr(addr[3])<<16 | MACAddr(addr[4])<<8 | MACAddr(addr[5])
}

// ParseMAC parses s as an IEEE 802 MAC-48 address using one of the formats
// accepted by [net.ParseMAC].
func ParseMAC(s string) (MACAddr, error) {
	hw, err := net.ParseMAC(s)
	if err != nil {
		return 0, err
	}
	mac, ok := MACAddrFromSlice(hw)
	if !ok {
		return 0, &net.AddrError{Err: "not a MAC-48 address", Addr: s}
	}
	return mac, nil
}

// AsSlice returns a MAC address in its 6-byte representation.
func (p MACAddr) AsSlice() []byte {
	mac := [6]byte{
		byte(p >> 40), byte(p >> 32), byte(p >> 24),
		byte(p >> 16), byte(p >> 8), byte(p),
	}
	return mac[:]
}

// String returns net.HardwareAddr(p.AsSlice()).String().
func (p MACAddr) String() string {
	return net.HardwareAddr(p.AsSlice()).String()
}

// IPMAC is a hashable tuple of an IP address and a MAC address suitable for use as a map key.
type IPMAC struct {
	ip  netip.Addr
	mac MACAddr
}

// IPMACFrom returns an [IPMAC] with the provided IP and MAC addresses.
func IPMACFrom(ip netip.Addr, mac MACAddr) IPMAC {
	return IPMAC{ip: ip, mac: mac}
}

func (i IPMAC) String() string {
	return i.ip.String() + " " + i.mac.String()
}

func (i IPMAC) IP() netip.Addr {
	return i.ip
}

func (i IPMAC) MAC() MACAddr {
	return i.mac
}
