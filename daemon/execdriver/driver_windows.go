package execdriver

import "github.com/docker/docker/pkg/nat"

// Network settings of the container
type Network struct {
	Interface   *NetworkInterface `json:"interface"`
	ContainerID string            `json:"container_id"` // id of the container to join network.
}

// NetworkInterface contains network configs for a driver
type NetworkInterface struct {
	MacAddress string `json:"mac"`
	Bridge     string `json:"bridge"`
	IPAddress  string `json:"ip"`

	// PortBindings is the port mapping between the exposed port in the
	// container and the port on the host.
	PortBindings nat.PortMap `json:"port_bindings"`
}
