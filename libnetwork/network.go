/*
Package libnetwork provides the basic functionality and extension points to
create network namespaces and allocate interfaces for containers to use.

    // Create a new controller instance
    controller := libnetwork.New()

    // This option is only needed for in-tree drivers. Plugins(in future) will get
    // their options through plugin infrastructure.
    option := options.Generic{}
    driver, err := controller.NewNetworkDriver("simplebridge", option)
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
    _, sinfo, err := network.CreateEndpoint("Endpoint1", networkNamespace.Key(), "")
    if err != nil {
	    return
    }
*/
package libnetwork

import (
	"sync"

	"github.com/docker/docker/pkg/common"
	"github.com/docker/libnetwork/driverapi"
)

// NetworkController provides the interface for controller instance which manages
// networks.
type NetworkController interface {
	NewNetworkDriver(networkType string, options interface{}) (*NetworkDriver, error)
	// Create a new network. The options parameter carry driver specific options.
	// Labels support will be added in the near future.
	NewNetwork(d *NetworkDriver, name string, options interface{}) (Network, error)
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
	CreateEndpoint(name string, sboxKey string, options interface{}) (Endpoint, *driverapi.SandboxInfo, error)

	// Delete the network.
	Delete() error
}

// Endpoint represents a logical connection between a network and a sandbox.
type Endpoint interface {
	// A system generated id for this network.
	ID() string

	// Delete endpoint.
	Delete() error
}

// NetworkDriver provides a reference to driver and way to push driver specific config
type NetworkDriver struct {
	networkType    string
	internalDriver driverapi.Driver
}

type endpoint struct {
	name        string
	id          driverapi.UUID
	network     *network
	sandboxInfo *driverapi.SandboxInfo
}

type network struct {
	ctrlr       *controller
	name        string
	networkType string
	id          driverapi.UUID
	driver      *NetworkDriver
	endpoints   endpointTable
	sync.Mutex
}

type networkTable map[driverapi.UUID]*network
type endpointTable map[driverapi.UUID]*endpoint

type controller struct {
	networks networkTable
	drivers  driverTable
	sync.Mutex
}

// New creates a new instance of network controller.
func New() NetworkController {
	return &controller{networkTable{}, enumerateDrivers(), sync.Mutex{}}
}

func (c *controller) NewNetworkDriver(networkType string, options interface{}) (*NetworkDriver, error) {
	d, ok := c.drivers[networkType]
	if !ok {
		return nil, NetworkTypeError(networkType)
	}

	if err := d.Config(options); err != nil {
		return nil, err
	}

	return &NetworkDriver{networkType: networkType, internalDriver: d}, nil
}

// NewNetwork creates a new network of the specified NetworkDriver. The options
// are driver specific and modeled in a generic way.
func (c *controller) NewNetwork(nd *NetworkDriver, name string, options interface{}) (Network, error) {
	if nd == nil {
		return nil, ErrNilNetworkDriver
	}

	c.Lock()
	for _, n := range c.networks {
		if n.name == name {
			c.Unlock()
			return nil, NetworkNameError(name)
		}
	}
	c.Unlock()

	network := &network{
		name:   name,
		id:     driverapi.UUID(common.GenerateRandomID()),
		ctrlr:  c,
		driver: nd}
	network.endpoints = make(endpointTable)

	d := network.driver.internalDriver
	if d == nil {
		return nil, ErrInvalidNetworkDriver
	}

	if err := d.CreateNetwork(network.id, options); err != nil {
		return nil, err
	}

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

	return n.driver.networkType
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

	d := n.driver.internalDriver
	err = d.DeleteNetwork(n.id)
	return err
}

func (n *network) CreateEndpoint(name string, sboxKey string, options interface{}) (Endpoint, *driverapi.SandboxInfo, error) {
	ep := &endpoint{name: name}
	ep.id = driverapi.UUID(common.GenerateRandomID())
	ep.network = n

	d := n.driver.internalDriver
	sinfo, err := d.CreateEndpoint(n.id, ep.id, sboxKey, options)
	if err != nil {
		return nil, nil, err
	}

	ep.sandboxInfo = sinfo
	n.Lock()
	n.endpoints[ep.id] = ep
	n.Unlock()
	return ep, sinfo, nil
}

func (ep *endpoint) ID() string {
	return string(ep.id)
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

	d := n.driver.internalDriver
	err = d.DeleteEndpoint(n.id, ep.id)
	return err
}
