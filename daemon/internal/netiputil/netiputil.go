package netiputil

import (
	"net"
	"net/netip"

	"github.com/moby/moby/v2/daemon/libnetwork/ipbits"
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
	if n == nil {
		return netip.Prefix{}, false
	}
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

// AddrPortFromNet converts a net.Addr into a netip.AddrPort.
func AddrPortFromNet(addr net.Addr) netip.AddrPort {
	if a, ok := addr.(interface{ AddrPort() netip.AddrPort }); ok {
		return a.AddrPort()
	}
	return netip.AddrPort{}
}

// LastAddr returns the last address of prefix 'p'.
func LastAddr(p netip.Prefix) netip.Addr {
	return ipbits.Add(p.Addr().Prev(), 1, uint(p.Addr().BitLen()-p.Bits()))
}

// PrefixCompare two prefixes and return a negative, 0, or a positive integer as
// required by [slices.SortFunc]. When two prefixes with the same address is
// provided, the shortest one will be sorted first.
func PrefixCompare(a, b netip.Prefix) int {
	cmp := a.Addr().Compare(b.Addr())
	if cmp != 0 {
		return cmp
	}
	return a.Bits() - b.Bits()
}

// PrefixAfter returns the prefix of size 'sz' right after 'prev'.
func PrefixAfter(prev netip.Prefix, sz int) netip.Prefix {
	s := min(prev.Bits(), sz)
	addr := ipbits.Add(prev.Addr(), 1, uint(prev.Addr().BitLen()-s))
	if addr.IsUnspecified() {
		return netip.Prefix{}
	}
	return netip.PrefixFrom(addr, sz).Masked()
}

// Unmap returns p with any IPv4-mapped IPv6 address prefix removed, per the
// quirky semantics that have been Hyrum's Law'ed into our Engine API contract:
// the subnet mask is truncated to the length of the unmapped address.
// For example:
//   - "::ffff:1.2.3.4/24" -> 1.2.3.4/0
//   - "::ffff:1.2.3.4/120" -> 1.2.3.4/24
func Unmap(p netip.Prefix) netip.Prefix {
	bits := p.Addr().Unmap().BitLen() - p.Addr().BitLen() + p.Bits()
	return netip.PrefixFrom(p.Addr().Unmap(), max(0, bits))
}

// ParseCIDR parses s as an IP address prefix, like [netip.ParsePrefix],
// unmapping any IPv4-mapped IPv6 address prefixes using [Unmap].
func ParseCIDR(s string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(s)
	return Unmap(prefix), err
}

// MaybeParse decorates a parse function to return no error when parsing the
// empty string.
func MaybeParse[T any](parse func(string) (T, error)) func(string) (T, error) {
	return func(s string) (T, error) {
		var zero T
		if s == "" {
			return zero, nil
		}
		return parse(s)
	}
}

func MaybeParseAddr(addr string) (netip.Addr, error) {
	return MaybeParse(netip.ParseAddr)(addr)
}

func MaybeParsePrefix(prefix string) (netip.Prefix, error) {
	return MaybeParse(netip.ParsePrefix)(prefix)
}

func MaybeParseCIDR(cidr string) (netip.Prefix, error) {
	return MaybeParse(ParseCIDR)(cidr)
}
