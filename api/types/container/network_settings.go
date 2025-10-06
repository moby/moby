package container

import (
	"github.com/moby/moby/api/types/network"
)

// NetworkSettings exposes the network settings in the api
type NetworkSettings struct {
	SandboxID  string // SandboxID uniquely represents a container's network stack
	SandboxKey string // SandboxKey identifies the sandbox

	// Ports is a collection of [network.PortBinding] indexed by [network.Port]
	Ports network.PortMap

	Networks map[string]*network.EndpointSettings
}

// NetworkSettingsSummary provides a summary of container's networks
// in /containers/json
type NetworkSettingsSummary struct {
	Networks map[string]*network.EndpointSettings
}
