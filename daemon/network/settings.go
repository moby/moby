package network

import "github.com/docker/docker/nat"

type Address struct {
	Addr      string
	PrefixLen int
}

type Settings struct {
	Bridge                 string
	EndpointID             string
	Gateway                string
	GlobalIPv6Address      string
	GlobalIPv6PrefixLen    int
	HairpinMode            bool
	IPAddress              string
	IPPrefixLen            int
	IPv6Gateway            string
	LinkLocalIPv6Address   string
	LinkLocalIPv6PrefixLen int
	MacAddress             string
	NetworkID              string
	PortMapping            map[string]map[string]string // Deprecated
	Ports                  nat.PortMap
	SandboxKey             string
	SecondaryIPAddresses   []Address
	SecondaryIPv6Addresses []Address
}
