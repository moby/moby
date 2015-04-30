package libnetwork

import (
	"sync"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/types"
)

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
	CreateEndpoint(name string, options interface{}) (Endpoint, error)

	// Delete the network.
	Delete() error

	// Endpoints returns the list of Endpoint(s) in this network.
	Endpoints() []Endpoint

	// WalkEndpoints uses the provided function to walk the Endpoints
	WalkEndpoints(walker EndpointWalker)

	// EndpointByName returns the Endpoint which has the passed name, if it exists otherwise nil is returned
	EndpointByName(name string) Endpoint

	// EndpointByID returns the Endpoint which has the passed id, if it exists otherwise nil is returned
	EndpointByID(id string) Endpoint
}

// EndpointWalker is a client provided function which will be used to walk the Endpoints.
// When the function returns true, the walk will stop.
type EndpointWalker func(ep Endpoint) bool

type network struct {
	ctrlr       *controller
	name        string
	networkType string
	id          types.UUID
	driver      driverapi.Driver
	endpoints   endpointTable
	sync.Mutex
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

func (n *network) CreateEndpoint(name string, options interface{}) (Endpoint, error) {
	ep := &endpoint{name: name}
	ep.id = types.UUID(stringid.GenerateRandomID())
	ep.network = n

	d := n.driver
	sinfo, err := d.CreateEndpoint(n.id, ep.id, options)
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
	list := make([]Endpoint, 0, len(n.endpoints))
	for _, e := range n.endpoints {
		list = append(list, e)
	}

	return list
}

func (n *network) WalkEndpoints(walker EndpointWalker) {
	for _, e := range n.Endpoints() {
		if walker(e) {
			return
		}
	}
}

func (n *network) EndpointByName(name string) Endpoint {
	var e Endpoint

	if name != "" {
		s := func(current Endpoint) bool {
			if current.Name() == name {
				e = current
				return true
			}
			return false
		}

		n.WalkEndpoints(s)
	}

	return e
}

func (n *network) EndpointByID(id string) Endpoint {
	n.Lock()
	defer n.Unlock()
	if e, ok := n.endpoints[types.UUID(id)]; ok {
		return e
	}
	return nil
}
