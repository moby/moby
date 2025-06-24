package portmapperapi

import (
	"net"
	"net/netip"

	"github.com/docker/docker/daemon/libnetwork/types"
)

type PortBindingReq struct {
	types.PortBinding
	// ChildHostIP is a temporary field used to pass the host IP address as
	// seen from the daemon. (It'll be removed once the portmapper API is
	// implemented).
	ChildHostIP net.IP `json:"-"`
	// DisableNAT is a temporary field used to indicate whether the port is
	// mapped on the host or not. (It'll be removed once the portmapper API is
	// implemented).
	DisableNAT bool `json:"-"`
}

// Compare defines an ordering over PortBindingReq such that bindings that
// differ only in host IP are adjacent (those bindings should be allocated the
// same port).
//
// Port bindings are first sorted by their mapper, then:
//   - exact host ports are placed before ranges (in case exact ports fall within
//     ranges, giving a better chance of allocating the exact ports), then
//   - same container port are adjacent (lowest ports first), then
//   - same protocols are adjacent (tcp < udp < sctp), then
//   - same host ports or ranges are adjacent, then
//   - ordered by container IP (then host IP, if set).
func (pbReq PortBindingReq) Compare(other PortBindingReq) int {
	if pbReq.DisableNAT != other.DisableNAT {
		if pbReq.DisableNAT {
			return 1 // NAT disabled bindings come last
		}
		return -1
	}
	// Exact host port < host port range.
	aIsRange := pbReq.HostPort == 0 || pbReq.HostPort != pbReq.HostPortEnd
	bIsRange := other.HostPort == 0 || other.HostPort != other.HostPortEnd
	if aIsRange != bIsRange {
		if aIsRange {
			return 1
		}
		return -1
	}
	if pbReq.Port != other.Port {
		return int(pbReq.Port) - int(other.Port)
	}
	if pbReq.Proto != other.Proto {
		return int(pbReq.Proto) - int(other.Proto)
	}
	if pbReq.HostPort != other.HostPort {
		return int(pbReq.HostPort) - int(other.HostPort)
	}
	if pbReq.HostPortEnd != other.HostPortEnd {
		return int(pbReq.HostPortEnd) - int(other.HostPortEnd)
	}
	aHostIP, _ := netip.AddrFromSlice(pbReq.HostIP)
	bHostIP, _ := netip.AddrFromSlice(other.HostIP)
	if c := aHostIP.Unmap().Compare(bHostIP.Unmap()); c != 0 {
		return c
	}
	aIP, _ := netip.AddrFromSlice(pbReq.IP)
	bIP, _ := netip.AddrFromSlice(other.IP)
	return aIP.Unmap().Compare(bIP.Unmap())
}
