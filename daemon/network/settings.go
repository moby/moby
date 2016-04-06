package network

import (
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/docker/go-connections/nat"
)

// Settings stores configuration details about the daemon network config
// TODO Windows. Many of these fields can be factored out.,
type Settings struct {
	Bridge                 string
	SandboxID              string
	HairpinMode            bool
	LinkLocalIPv6Address   string
	LinkLocalIPv6PrefixLen int
	Networks               map[string]*networktypes.EndpointSettings
	Ports                  nat.PortMap
	SandboxKey             string
	SecondaryIPAddresses   []networktypes.Address
	SecondaryIPv6Addresses []networktypes.Address
	IsAnonymousEndpoint    bool
}
