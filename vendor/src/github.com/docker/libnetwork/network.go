package libnetwork

import (
	"encoding/json"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/datastore"
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
	endpointCnt uint64
	endpoints   endpointTable
	generic     options.Generic
	dbIndex     uint64
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

func (n *network) Key() []string {
	n.Lock()
	defer n.Unlock()
	return []string{datastore.NetworkKeyPrefix, string(n.id)}
}

func (n *network) KeyPrefix() []string {
	return []string{datastore.NetworkKeyPrefix}
}

func (n *network) Value() []byte {
	n.Lock()
	defer n.Unlock()
	b, err := json.Marshal(n)
	if err != nil {
		return nil
	}
	return b
}

func (n *network) Index() uint64 {
	n.Lock()
	defer n.Unlock()
	return n.dbIndex
}

func (n *network) SetIndex(index uint64) {
	n.Lock()
	n.dbIndex = index
	n.Unlock()
}

func (n *network) EndpointCnt() uint64 {
	n.Lock()
	defer n.Unlock()
	return n.endpointCnt
}

func (n *network) IncEndpointCnt() {
	n.Lock()
	n.endpointCnt++
	n.Unlock()
}

func (n *network) DecEndpointCnt() {
	n.Lock()
	n.endpointCnt--
	n.Unlock()
}

// TODO : Can be made much more generic with the help of reflection (but has some golang limitations)
func (n *network) MarshalJSON() ([]byte, error) {
	netMap := make(map[string]interface{})
	netMap["name"] = n.name
	netMap["id"] = string(n.id)
	netMap["networkType"] = n.networkType
	netMap["endpointCnt"] = n.endpointCnt
	netMap["enableIPv6"] = n.enableIPv6
	netMap["generic"] = n.generic
	return json.Marshal(netMap)
}

// TODO : Can be made much more generic with the help of reflection (but has some golang limitations)
func (n *network) UnmarshalJSON(b []byte) (err error) {
	var netMap map[string]interface{}
	if err := json.Unmarshal(b, &netMap); err != nil {
		return err
	}
	n.name = netMap["name"].(string)
	n.id = types.UUID(netMap["id"].(string))
	n.networkType = netMap["networkType"].(string)
	n.endpointCnt = uint64(netMap["endpointCnt"].(float64))
	n.enableIPv6 = netMap["enableIPv6"].(bool)
	if netMap["generic"] != nil {
		n.generic = netMap["generic"].(map[string]interface{})
	}
	return nil
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

	n.Lock()
	ctrlr := n.ctrlr
	n.Unlock()

	ctrlr.Lock()
	_, ok := ctrlr.networks[n.id]
	ctrlr.Unlock()

	if !ok {
		return &UnknownNetworkError{name: n.name, id: string(n.id)}
	}

	numEps := n.EndpointCnt()
	if numEps != 0 {
		return &ActiveEndpointsError{name: n.name, id: string(n.id)}
	}

	// deleteNetworkFromStore performs an atomic delete operation and the network.endpointCnt field will help
	// prevent any possible race between endpoint join and network delete
	if err = ctrlr.deleteNetworkFromStore(n); err != nil {
		if err == datastore.ErrKeyModified {
			return types.InternalErrorf("operation in progress. delete failed for network %s. Please try again.")
		}
		return err
	}

	if err = n.deleteNetwork(); err != nil {
		return err
	}

	return nil
}

func (n *network) deleteNetwork() error {
	n.Lock()
	id := n.id
	d := n.driver
	n.ctrlr.Lock()
	delete(n.ctrlr.networks, id)
	n.ctrlr.Unlock()
	n.Unlock()

	if err := d.DeleteNetwork(n.id); err != nil {
		// Forbidden Errors should be honored
		if _, ok := err.(types.ForbiddenError); ok {
			n.ctrlr.Lock()
			n.ctrlr.networks[n.id] = n
			n.ctrlr.Unlock()
			return err
		}
		log.Warnf("driver error deleting network %s : %v", n.name, err)
	}
	return nil
}

func (n *network) addEndpoint(ep *endpoint) error {
	var err error
	n.Lock()
	n.endpoints[ep.id] = ep
	d := n.driver
	n.Unlock()

	defer func() {
		if err != nil {
			n.Lock()
			delete(n.endpoints, ep.id)
			n.Unlock()
		}
	}()

	err = d.CreateEndpoint(n.id, ep.id, ep, ep.generic)
	if err != nil {
		return err
	}
	return nil
}

func (n *network) CreateEndpoint(name string, options ...EndpointOption) (Endpoint, error) {
	var err error
	if name == "" {
		return nil, ErrInvalidName(name)
	}

	if _, err = n.EndpointByName(name); err == nil {
		return nil, types.ForbiddenErrorf("service endpoint with name %s already exists", name)
	}

	ep := &endpoint{name: name, iFaces: []*endpointInterface{}, generic: make(map[string]interface{})}
	ep.id = types.UUID(stringid.GenerateRandomID())
	ep.network = n
	ep.processOptions(options...)

	n.Lock()
	ctrlr := n.ctrlr
	n.Unlock()

	n.IncEndpointCnt()
	if err = ctrlr.updateNetworkToStore(n); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			n.DecEndpointCnt()
			if err = ctrlr.updateNetworkToStore(n); err != nil {
				log.Warnf("endpoint count cleanup failed when updating network for %s : %v", name, err)
			}
		}
	}()
	if err = n.addEndpoint(ep); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if e := ep.Delete(); ep != nil {
				log.Warnf("cleaning up endpoint failed %s : %v", name, e)
			}
		}
	}()

	if err = ctrlr.updateEndpointToStore(ep); err != nil {
		return nil, err
	}

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

func (n *network) isGlobalScoped() (bool, error) {
	n.Lock()
	c := n.ctrlr
	n.Unlock()
	return c.isDriverGlobalScoped(n.networkType)
}
