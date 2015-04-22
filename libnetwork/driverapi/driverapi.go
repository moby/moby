package driverapi

import (
	"errors"

	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

var (
	// ErrEndpointExists is returned if more than one endpoint is added to the network
	ErrEndpointExists = errors.New("Endpoint already exists (Only one endpoint allowed)")
	// ErrNoNetwork is returned if no network with the specified id exists
	ErrNoNetwork = errors.New("No network exists")
	// ErrNoEndpoint is returned if no endpoint with the specified id exists
	ErrNoEndpoint = errors.New("No endpoint exists")
)

// Driver is an interface that every plugin driver needs to implement.
type Driver interface {
	// Push driver specific config to the driver
	Config(config interface{}) error

	// CreateNetwork invokes the driver method to create a network passing
	// the network id and network specific config. The config mechanism will
	// eventually be replaced with labels which are yet to be introduced.
	CreateNetwork(nid types.UUID, config interface{}) error

	// DeleteNetwork invokes the driver method to delete network passing
	// the network id.
	DeleteNetwork(nid types.UUID) error

	// CreateEndpoint invokes the driver method to create an endpoint
	// passing the network id, endpoint id, sandbox key and driver
	// specific config. The config mechanism will eventually be replaced
	// with labels which are yet to be introduced.
	CreateEndpoint(nid, eid types.UUID, key string, config interface{}) (*sandbox.Info, error)

	// DeleteEndpoint invokes the driver method to delete an endpoint
	// passing the network id and endpoint id.
	DeleteEndpoint(nid, eid types.UUID) error

	// Type returns the the type of this driver, the network type this driver manages
	Type() string
}
