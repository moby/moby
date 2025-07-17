package portmapperapi

import (
	"os"

	"github.com/docker/docker/daemon/libnetwork/types"
)

type ProxyManager interface {
	// StartProxy starts the proxy process for the given port binding.
	StartProxy(pb types.PortBinding, listenSock *os.File) (Proxy, error)
}

// Proxy represents the userland proxy started for a specific port binding.
type Proxy interface {
	// Stop stops the userland proxy process.
	Stop() error
}
