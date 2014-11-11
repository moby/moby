package network

import "github.com/docker/docker/nat"

type Settings struct {
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
	PortMapping            map[string]map[string]string // Deprecated
	Ports                  nat.PortMap
	HairpinMode            bool
}
