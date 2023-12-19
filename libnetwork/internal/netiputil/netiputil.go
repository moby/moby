package netiputil

import (
	"net"
	"net/netip"

	"github.com/docker/docker/libnetwork/ipbits"
)

// ToIPNet converts p into a *net.IPNet, returning nil if p is not valid.
func ToIPNet(p netip.Prefix) *net.IPNet {
	if !p.IsValid() {
		return nil
	}
	return &net.IPNet{
		IP:   p.Addr().AsSlice(),
		Mask: net.CIDRMask(p.Bits(), p.Addr().BitLen()),
	}
}

// ToPrefix converts n into a netip.Prefix. If n is not a valid IPv4 or IPV6
// address, ToPrefix returns netip.Prefix{}, false.
func ToPrefix(n *net.IPNet) (netip.Prefix, bool) {
	if ll := len(n.Mask); ll != net.IPv4len && ll != net.IPv6len {
		return netip.Prefix{}, false
	}

	addr, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}

	ones, bits := n.Mask.Size()
	if ones == 0 && bits == 0 {
		return netip.Prefix{}, false
	}

	return netip.PrefixFrom(addr.Unmap(), ones), true
}

// HostID masks out the 'bits' most-significant bits of addr. The result is
// undefined if bits > addr.BitLen().
func HostID(addr netip.Addr, bits uint) uint64 {
	return ipbits.Field(addr, bits, uint(addr.BitLen()))
}

// SubnetRange returns the amount to add to network.Addr() in order to yield the
// first and last addresses in subnet, respectively.
func SubnetRange(network, subnet netip.Prefix) (start, end uint64) {
	start = HostID(subnet.Addr(), uint(network.Bits()))
	end = start + (1 << uint64(subnet.Addr().BitLen()-subnet.Bits())) - 1
	return start, end
}
