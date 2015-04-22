/*
Package libnetwork provides the basic functionality and extension points to
create network namespaces and allocate interfaces for containers to use.

    // Create a new controller instance
    controller := libnetwork.New()

    // This option is only needed for in-tree drivers. Plugins(in future) will get
    // their options through plugin infrastructure.
    option := options.Generic{}
    driver, err := controller.NewNetworkDriver("bridge", option)
    if err != nil {
        return
    }

    netOptions := options.Generic{}
    // Create a network for containers to join.
    network, err := controller.NewNetwork(driver, "network1", netOptions)
    if err != nil {
    	return
    }

    // For a new container: create a sandbox instance (providing a unique key).
    // For linux it is a filesystem path
    networkPath := "/var/lib/docker/.../4d23e"
    networkNamespace, err := sandbox.NewSandbox(networkPath)
    if err != nil {
	    return
    }

    // For each new container: allocate IP and interfaces. The returned network
    // settings will be used for container infos (inspect and such), as well as
    // iptables rules for port publishing.
    ep, err := network.CreateEndpoint("Endpoint1", networkNamespace.Key(), nil)
    if err != nil {
	    return
    }
*/
package libnetwork

import (
	"sync"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

// NetworkController provides the interface for controller instance which manages
// networks.
type NetworkController interface {
	// ConfigureNetworkDriver applies the passed options to the driver instance for the specified network type
	ConfigureNetworkDriver(networkType string, options interface{}) error
	// Create a new network. The options parameter carries network specific options.
	// Labels support will be added in the near future.
	NewNetwork(networkType, name string, options interface{}) (Network, error)
}

// A Network represents a logical connectivity zone that containers may
// join using the Link method. A Network is managed by a specific driver.
type Network interface {
	// A user chosen name for this network.
	Name() string

	// A system generated id for this network.
	ID() string

	// The type of network, which corresponds to its managing driver.
	Type() string

	// Create a new endpoint to this network symbolically identified by the
	// specified unique name. The options parameter carry driver specific options.
	// Labels support will be added in the near future.
	CreateEndpoint(name string, sboxKey string, options interface{}) (Endpoint, error)

	// Endpoints returns the list of Endpoint in this network.
	Endpoints() []Endpoint

	// Delete the network.
	Delete() error
}

// Endpoint represents a logical connection between a network and a sandbox.
type Endpoint interface {
	// A system generated id for this endpoint.
	ID() string

	// Name returns the name of this endpoint.
	Name() string

	// Network returns the name of the network to which this endpoint is attached.
	Network() string

	// SandboxInfo returns the sandbox information for this endpoint.
	SandboxInfo() *sandbox.Info

	// Delete and detaches this endpoint from the network.
	Delete() error
}

type endpoint struct {
	name        string
	id          types.UUID
	network     *network
	sandboxInfo *sandbox.Info
}

type network struct {
	ctrlr       *controller
	name        string
	networkType string
	id          types.UUID
	driver      driverapi.Driver
	endpoints   endpointTable
	sync.Mutex
}

type networkTable map[types.UUID]*network
type endpointTable map[types.UUID]*endpoint

type controller struct {
	networks networkTable
	drivers  driverTable
	sync.Mutex
}

// New creates a new instance of network controller.
func New() NetworkController {
	return &controller{networkTable{}, enumerateDrivers(), sync.Mutex{}}
}

func (c *controller) ConfigureNetworkDriver(networkType string, options interface{}) error {
	d, ok := c.drivers[networkType]
	if !ok {
		return NetworkTypeError(networkType)
	}
	return d.Config(options)
}

// NewNetwork creates a new network of the specified network type. The options
// are network specific and modeled in a generic way.
func (c *controller) NewNetwork(networkType, name string, options interface{}) (Network, error) {
	// Check if a driver for the specified network type is available
	d, ok := c.drivers[networkType]
	if !ok {
		return nil, ErrInvalidNetworkDriver
	}

	// Check if a network already exists with the specified network name
	c.Lock()
	for _, n := range c.networks {
		if n.name == name {
			c.Unlock()
			return nil, NetworkNameError(name)
		}
	}
	c.Unlock()

	// Construct the network object
	network := &network{
		name:      name,
		id:        types.UUID(stringid.GenerateRandomID()),
		ctrlr:     c,
		driver:    d,
		endpoints: endpointTable{},
	}

	// Create the network
	if err := d.CreateNetwork(network.id, options); err != nil {
		return nil, err
	}

	// Store the network handler in controller
	c.Lock()
	c.networks[network.id] = network
	c.Unlock()

	return network, nil
}

func (n *network) Name() string {
	return n.name
}

func (n *network) ID() string {
	return string(n.id)
}

func (n *network) Type() string {
	if n.driver == nil {
		return ""
	}

	return n.driver.Type()
}

func (n *network) Delete() error {
	var err error

	n.ctrlr.Lock()
	_, ok := n.ctrlr.networks[n.id]
	if !ok {
		n.ctrlr.Unlock()
		return &UnknownNetworkError{name: n.name, id: string(n.id)}
	}

	n.Lock()
	numEps := len(n.endpoints)
	n.Unlock()
	if numEps != 0 {
		n.ctrlr.Unlock()
		return &ActiveEndpointsError{name: n.name, id: string(n.id)}
	}

	delete(n.ctrlr.networks, n.id)
	n.ctrlr.Unlock()
	defer func() {
		if err != nil {
			n.ctrlr.Lock()
			n.ctrlr.networks[n.id] = n
			n.ctrlr.Unlock()
		}
	}()

	err = n.driver.DeleteNetwork(n.id)
	return err
}

func (n *network) CreateEndpoint(name string, sboxKey string, options interface{}) (Endpoint, error) {
	ep := &endpoint{name: name}
	ep.id = types.UUID(stringid.GenerateRandomID())
	ep.network = n

	d := n.driver
	sinfo, err := d.CreateEndpoint(n.id, ep.id, sboxKey, options)
	if err != nil {
		return nil, err
	}

	ep.sandboxInfo = sinfo
	n.Lock()
	n.endpoints[ep.id] = ep
	n.Unlock()
	return ep, nil
}

func (n *network) Endpoints() []Endpoint {
	n.Lock()
	defer n.Unlock()

	list := make([]Endpoint, len(n.endpoints))

	idx := 0
	for _, e := range n.endpoints {
		list[idx] = e
		idx++
	}

	return list
}

func (ep *endpoint) ID() string {
	return string(ep.id)
}

func (ep *endpoint) Name() string {
	return ep.name
}

func (ep *endpoint) Network() string {
	return ep.network.name
}

func (ep *endpoint) SandboxInfo() *sandbox.Info {
	if ep.sandboxInfo == nil {
		return nil
	}
	return ep.sandboxInfo.GetCopy()
}

func (ep *endpoint) Delete() error {
	var err error

	n := ep.network
	n.Lock()
	_, ok := n.endpoints[ep.id]
	if !ok {
		n.Unlock()
		return &UnknownEndpointError{name: ep.name, id: string(ep.id)}
	}

	delete(n.endpoints, ep.id)
	n.Unlock()
	defer func() {
		if err != nil {
			n.Lock()
			n.endpoints[ep.id] = ep
			n.Unlock()
		}
	}()

	err = n.driver.DeleteEndpoint(n.id, ep.id)
	return err
}
