package libnetwork

import (
	"sync"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
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
	CreateEndpoint(name string, options ...EndpointOption) (Endpoint, error)

	// Delete the network.
	Delete() error

	// Endpoints returns the list of Endpoint(s) in this network.
	Endpoints() []Endpoint

	// WalkEndpoints uses the provided function to walk the Endpoints
	WalkEndpoints(walker EndpointWalker)

	// EndpointByName returns the Endpoint which has the passed name. If not found, the error ErrNoSuchEndpoint is returned.
	EndpointByName(name string) (Endpoint, error)

	// EndpointByID returns the Endpoint which has the passed id. If not found, the error ErrNoSuchEndpoint is returned.
	EndpointByID(id string) (Endpoint, error)
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
	enableIPv6  bool
	endpoints   endpointTable
	generic     options.Generic
	sync.Mutex
}

func (n *network) Name() string {
	n.Lock()
	defer n.Unlock()

	return n.name
}

func (n *network) ID() string {
	n.Lock()
	defer n.Unlock()

	return string(n.id)
}

func (n *network) Type() string {
	n.Lock()
	defer n.Unlock()

	if n.driver == nil {
		return ""
	}

	return n.driver.Type()
}

// NetworkOption is a option setter function type used to pass varios options to
// NewNetwork method. The various setter functions of type NetworkOption are
// provided by libnetwork, they look like NetworkOptionXXXX(...)
type NetworkOption func(n *network)

// NetworkOptionGeneric function returns an option setter for a Generic option defined
// in a Dictionary of Key-Value pair
func NetworkOptionGeneric(generic map[string]interface{}) NetworkOption {
	return func(n *network) {
		n.generic = generic
		if _, ok := generic[netlabel.EnableIPv6]; ok {
			n.enableIPv6 = generic[netlabel.EnableIPv6].(bool)
		}
	}
}

func (n *network) processOptions(options ...NetworkOption) {
	for _, opt := range options {
		if opt != nil {
			opt(n)
		}
	}
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

func (n *network) CreateEndpoint(name string, options ...EndpointOption) (Endpoint, error) {
	if name == "" {
		return nil, ErrInvalidName(name)
	}
	ep := &endpoint{name: name, iFaces: []*endpointInterface{}, generic: make(map[string]interface{})}
	ep.id = types.UUID(stringid.GenerateRandomID())
	ep.network = n
	ep.processOptions(options...)

	d := n.driver
	err := d.CreateEndpoint(n.id, ep.id, ep, ep.generic)
	if err != nil {
		return nil, err
	}

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

func (n *network) EndpointByName(name string) (Endpoint, error) {
	if name == "" {
		return nil, ErrInvalidName(name)
	}
	var e Endpoint

	s := func(current Endpoint) bool {
		if current.Name() == name {
			e = current
			return true
		}
		return false
	}

	n.WalkEndpoints(s)

	if e == nil {
		return nil, ErrNoSuchEndpoint(name)
	}

	return e, nil
}

func (n *network) EndpointByID(id string) (Endpoint, error) {
	if id == "" {
		return nil, ErrInvalidID(id)
	}
	n.Lock()
	defer n.Unlock()
	if e, ok := n.endpoints[types.UUID(id)]; ok {
		return e, nil
	}
	return nil, ErrNoSuchEndpoint(id)
}
