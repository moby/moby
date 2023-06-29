package ipam

import (
	"net"
	"net/netip"

	"github.com/docker/docker/libnetwork/ipbits"
)

func toIPNet(p netip.Prefix) *net.IPNet {
	if !p.IsValid() {
		return nil
	}
	return &net.IPNet{
		IP:   p.Addr().AsSlice(),
		Mask: net.CIDRMask(p.Bits(), p.Addr().BitLen()),
	}
}

func hostID(addr netip.Addr, bits uint) uint64 {
	return ipbits.Field(addr, bits, uint(addr.BitLen()))
}

// subnetRange returns the amount to add to network.Addr() in order to yield the
// first and last addresses in subnet, respectively.
func subnetRange(network, subnet netip.Prefix) (start, end uint64) {
	start = hostID(subnet.Addr(), uint(network.Bits()))
	end = start + (1 << uint64(subnet.Addr().BitLen()-subnet.Bits())) - 1
	return start, end
}
