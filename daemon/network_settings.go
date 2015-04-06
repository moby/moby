package daemon

import (
	"github.com/docker/docker/nat"
)

// FIXME: move deprecated port stuff to nat to clean up the core.
type PortMapping map[string]string // Deprecated

type NetworkSettings struct {
	IPAddress              string
	IPPrefixLen            int
	MacAddress             string
	LinkLocalIPv6Address   string
	LinkLocalIPv6PrefixLen int
	GlobalIPv6Address      string
	GlobalIPv6PrefixLen    int
	Gateway                string
	IPv6Gateway            string
	Bridge                 string
	PortMapping            map[string]PortMapping // Deprecated
	Ports                  nat.PortMap
}
