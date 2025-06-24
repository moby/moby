package portmapperapi

import (
	"net"

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
