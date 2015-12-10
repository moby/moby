package network

import (
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/nat"
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
