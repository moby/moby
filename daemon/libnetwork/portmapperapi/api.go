package portmapperapi

import (
	"context"
	"net"
	"net/netip"
	"os"
	"strings"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

// Registerer provides a callback interface for registering port-mappers.
type Registerer interface {
	// Register provides a way for port-mappers to dynamically register with libnetwork.
	Register(name string, driver PortMapper) error
}

// PortMapper maps / unmaps container ports to host ports.
type PortMapper interface {
	// MapPorts takes a list of port binding requests, and returns a list of
	// PortBinding. Both lists MUST have the same size.
	//
	// Multiple port bindings are passed when they're all requesting the
	// same port range, or an ephemeral port, over multiple IP addresses and
	// all pointing to the same container port. In that case, the PortMapper
	// MUST assign the same HostPort for all IP addresses.
	//
	// When an ephemeral port, or a single port from a range is requested
	// MapPorts should attempt a few times to find a free port available
	// across all IP addresses.
	MapPorts(ctx context.Context, reqs []PortBindingReq, fwn Firewaller) ([]PortBinding, error)

	// UnmapPorts takes a list of port bindings to unmap.
	UnmapPorts(ctx context.Context, pbs []PortBinding, fwn Firewaller) error
}

type PortBindingReq struct {
	types.PortBinding
	// Mapper is the name of the port mapper used to process this PortBindingReq.
	Mapper string
	// ChildHostIP is a temporary field used to pass the host IP address as
	// seen from the daemon. (It'll be removed once the portmapper API is
	// implemented).
	ChildHostIP net.IP `json:"-"`
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
	if pbReq.Mapper != other.Mapper {
		return strings.Compare(pbReq.Mapper, other.Mapper)
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

type PortBinding struct {
	types.PortBinding
	// Mapper is the name of the port mapper used to process this PortBinding.
	Mapper string
	// BoundSocket is used to reserve a host port for the binding. If the
	// userland proxy is in-use, it's passed to the proxy when the proxy is
	// started, then it's closed and set to nil here.
	BoundSocket *os.File `json:"-"`
	// ChildHostIP is the host IP address, as seen from the daemon. This
	// is normally the same as PortBinding.HostIP but, in rootless mode, it
	// will be an address in the rootless network namespace. RootlessKit
	// binds the port on the real (parent) host address and maps it to the
	// same port number on the address dockerd sees in the child namespace.
	// So, for example, docker-proxy and DNAT rules need to use the child
	// namespace's host address. (PortBinding.HostIP isn't replaced by the
	// child address, because it's stored as user-config and the child
	// address may change if RootlessKit is configured differently.)
	ChildHostIP net.IP `json:"-"`
	// PortDriverRemove is a function that will inform the RootlessKit
	// port driver about removal of a port binding, or nil.
	PortDriverRemove func() error `json:"-"`
	// StopProxy is a function to stop the userland proxy for this binding,
	// if a proxy has been started - else nil.
	StopProxy func() error `json:"-"`
	// RootlesskitUnsupported is set to true when the port binding is not
	// supported by the port driver of RootlessKit.
	RootlesskitUnsupported bool `json:"-"`
}

// ChildPortBinding is pb.PortBinding, with the host address the daemon
// will see - which, in rootless mode, will be an address in the RootlessKit's
// child namespace (see PortBinding.ChildHostIP).
func (pb PortBinding) ChildPortBinding() types.PortBinding {
	res := pb.PortBinding
	res.HostIP = pb.ChildHostIP
	return res
}
