package portmapperapi

import (
	"context"
	"net"
	"os"

	"github.com/docker/docker/daemon/libnetwork/types"
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
	// Proxy is the userland proxy for this binding, if a proxy has been
	// started - else nil.
	Proxy Proxy `json:"-"`
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
