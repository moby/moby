package portmapperapi

import (
	"net"
	"os"

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

type PortBinding struct {
	types.PortBinding
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
