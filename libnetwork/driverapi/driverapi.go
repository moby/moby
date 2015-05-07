package driverapi

import (
	"errors"
	"fmt"

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
	// ErrNotImplemented is returned when a Driver has not implemented an API yet
	ErrNotImplemented = errors.New("The API is not implemented yet")
)

// Driver is an interface that every plugin driver needs to implement.
type Driver interface {
	// Push driver specific config to the driver
	Config(options map[string]interface{}) error

	// CreateNetwork invokes the driver method to create a network passing
	// the network id and network specific config. The config mechanism will
	// eventually be replaced with labels which are yet to be introduced.
	CreateNetwork(nid types.UUID, options map[string]interface{}) error

	// DeleteNetwork invokes the driver method to delete network passing
	// the network id.
	DeleteNetwork(nid types.UUID) error

	// CreateEndpoint invokes the driver method to create an endpoint
	// passing the network id, endpoint id and driver
	// specific config. The config mechanism will eventually be replaced
	// with labels which are yet to be introduced.
	CreateEndpoint(nid, eid types.UUID, options map[string]interface{}) (*sandbox.Info, error)

	// DeleteEndpoint invokes the driver method to delete an endpoint
	// passing the network id and endpoint id.
	DeleteEndpoint(nid, eid types.UUID) error

	// EndpointInfo retrieves from the driver the operational data related to the specified endpoint
	EndpointInfo(nid, eid types.UUID) (map[string]interface{}, error)

	// Join method is invoked when a Sandbox is attached to an endpoint.
	Join(nid, eid types.UUID, sboxKey string, options map[string]interface{}) (*JoinInfo, error)

	// Leave method is invoked when a Sandbox detaches from an endpoint.
	Leave(nid, eid types.UUID, options map[string]interface{}) error

	// Type returns the the type of this driver, the network type this driver manages
	Type() string
}

// JoinInfo represents a set of resources that the driver has the ability to provide during
// join time.
type JoinInfo struct {
	HostsPath string
}

// ErrActiveRegistration represents an error when a driver is registered to a networkType that is previously registered
type ErrActiveRegistration string

// Error interface for ErrActiveRegistration
func (ar ErrActiveRegistration) Error() string {
	return fmt.Sprintf("Driver already registered for type %q", string(ar))
}

// DriverCallback provides a Callback interface for Drivers into LibNetwork
type DriverCallback interface {
	// RegisterDriver provides a way for Remote drivers to dynamically register new NetworkType and associate with a driver instance
	RegisterDriver(name string, driver Driver) error
}
