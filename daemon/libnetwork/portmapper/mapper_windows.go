package portmapper

import (
	"sync"

	"github.com/moby/moby/v2/daemon/libnetwork/portallocator"
)

// PortMapper manages the network address translation
type PortMapper struct {
	bridgeName string

	// udp:ip:port
	currentMappings map[string]*mapping
	lock            sync.Mutex

	proxyPath string

	allocator *portallocator.PortAllocator
}
