/*
Package libnetwork provides the basic functionality and extension points to
create network namespaces and allocate interfaces for containers to use.

        // Create a new controller instance
        controller, _err := libnetwork.New()

        // Select and configure the network driver
        networkType := "bridge"

        driverOptions := options.Generic{}
        genericOption := make(map[string]interface{})
        genericOption[netlabel.GenericData] = driverOptions
        err := controller.ConfigureNetworkDriver(networkType, genericOption)
        if err != nil {
                return
        }

        // Create a network for containers to join.
        // NewNetwork accepts Variadic optional arguments that libnetwork and Drivers can make of
        network, err := controller.NewNetwork(networkType, "network1")
        if err != nil {
                return
        }

        // For each new container: allocate IP and interfaces. The returned network
        // settings will be used for container infos (inspect and such), as well as
        // iptables rules for port publishing. This info is contained or accessible
        // from the returned endpoint.
        ep, err := network.CreateEndpoint("Endpoint1")
        if err != nil {
                return
        }

        // A container can join the endpoint by providing the container ID to the join
        // api which returns the sandbox key which can be used to access the sandbox
        // created for the container during join.
        // Join acceps Variadic arguments which will be made use of by libnetwork and Drivers
        _, err = ep.Join("container1",
                libnetwork.JoinOptionHostname("test"),
                libnetwork.JoinOptionDomainname("docker.io"))
        if err != nil {
                return
        }
*/
package libnetwork

import (
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
	"github.com/docker/swarm/pkg/store"
)

// NetworkController provides the interface for controller instance which manages
// networks.
type NetworkController interface {
	// ConfigureNetworkDriver applies the passed options to the driver instance for the specified network type
	ConfigureNetworkDriver(networkType string, options map[string]interface{}) error

	// Create a new network. The options parameter carries network specific options.
	// Labels support will be added in the near future.
	NewNetwork(networkType, name string, options ...NetworkOption) (Network, error)

	// Networks returns the list of Network(s) managed by this controller.
	Networks() []Network

	// WalkNetworks uses the provided function to walk the Network(s) managed by this controller.
	WalkNetworks(walker NetworkWalker)

	// NetworkByName returns the Network which has the passed name. If not found, the error ErrNoSuchNetwork is returned.
	NetworkByName(name string) (Network, error)

	// NetworkByID returns the Network which has the passed id. If not found, the error ErrNoSuchNetwork is returned.
	NetworkByID(id string) (Network, error)
}

// NetworkWalker is a client provided function which will be used to walk the Networks.
// When the function returns true, the walk will stop.
type NetworkWalker func(nw Network) bool

type sandboxData struct {
	sandbox sandbox.Sandbox
	refCnt  int
}

type networkTable map[types.UUID]*network
type endpointTable map[types.UUID]*endpoint
type sandboxTable map[string]*sandboxData

type controller struct {
	networks  networkTable
	drivers   driverTable
	sandboxes sandboxTable
	store     datastore.DataStore
	sync.Mutex
}

// New creates a new instance of network controller.
func New() (NetworkController, error) {
	c := &controller{
		networks:  networkTable{},
		sandboxes: sandboxTable{},
		drivers:   driverTable{}}
	if err := initDrivers(c); err != nil {
		return nil, err
	}

	if err := c.initDataStore(); err != nil {
		log.Errorf("Failed to Initialize Datastore : %v", err)
		// TODO : Should we fail if the initDataStore fail here ?
	}

	go c.watchNewNetworks()
	return c, nil
}

func (c *controller) initDataStore() error {
	/* TODO : Duh ! make this configurable */
	config := &datastore.StoreConfiguration{}
	config.Provider = "consul"
	config.Addrs = []string{"localhost:8500"}

	store, err := datastore.NewDataStore(config)
	if err != nil {
		return err
	}
	c.Lock()
	c.store = store
	c.Unlock()

	return nil
}

func (c *controller) ConfigureNetworkDriver(networkType string, options map[string]interface{}) error {
	c.Lock()
	d, ok := c.drivers[networkType]
	c.Unlock()
	if !ok {
		return NetworkTypeError(networkType)
	}
	return d.Config(options)
}

func (c *controller) RegisterDriver(networkType string, driver driverapi.Driver) error {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.drivers[networkType]; ok {
		return driverapi.ErrActiveRegistration(networkType)
	}
	c.drivers[networkType] = driver
	return nil
}

// NewNetwork creates a new network of the specified network type. The options
// are network specific and modeled in a generic way.
func (c *controller) NewNetwork(networkType, name string, options ...NetworkOption) (Network, error) {
	if name == "" {
		return nil, ErrInvalidName(name)
	}
	// Check if a driver for the specified network type is available
	c.Lock()
	d, ok := c.drivers[networkType]
	c.Unlock()
	if !ok {
		var err error
		d, err = c.loadDriver(networkType)
		if err != nil {
			return nil, err
		}
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

	network.processOptions(options...)
	// Create the network
	if err := d.CreateNetwork(network.id, network.generic); err != nil {
		return nil, err
	}

	// Store the network handler in controller
	c.Lock()
	c.networks[network.id] = network
	c.store.PutObjectAtomic(network)
	c.Unlock()

	return network, nil
}

func (c *controller) newNetworkFromStore(n *network) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.drivers[n.networkType]; !ok {
		log.Warnf("Network driver unavailable for type=%s. ignoring network updates for %s", n.Type(), n.Name())
		return
	}
	n.ctrlr = c
	n.driver = c.drivers[n.networkType]
	c.networks[n.id] = n
	// TODO : Populate n.endpoints back from endpoint dbstore
}

func (c *controller) watchNewNetworks() {
	c.Lock()
	store = c.store
	c.Unlock()

	store.KVStore().WatchRange(datastore.Key("network"), "", 0, func(kvi []store.KVEntry) {
		for _, kve := range kvi {
			var n network
			err := json.Unmarshal(kve.Value(), &n)
			if err != nil {
				log.Error(err)
				continue
			}
			n.dbIndex = kve.LastIndex()
			c.Lock()
			existing, ok := c.networks[n.id]
			c.Unlock()
			if ok && existing.dbIndex == n.dbIndex {
				// Skip any watch notification for a network that has not changed
				continue
			}
			fmt.Printf("WATCHED : %v = %v\n", kve.Key(), n)
			c.newNetworkFromStore(&n)
		}
	})
}

func (c *controller) Networks() []Network {
	c.Lock()
	defer c.Unlock()

	list := make([]Network, 0, len(c.networks))
	for _, n := range c.networks {
		list = append(list, n)
	}

	return list
}

func (c *controller) WalkNetworks(walker NetworkWalker) {
	for _, n := range c.Networks() {
		if walker(n) {
			return
		}
	}
}

func (c *controller) NetworkByName(name string) (Network, error) {
	if name == "" {
		return nil, ErrInvalidName(name)
	}
	var n Network

	s := func(current Network) bool {
		if current.Name() == name {
			n = current
			return true
		}
		return false
	}

	c.WalkNetworks(s)

	if n == nil {
		return nil, ErrNoSuchNetwork(name)
	}

	return n, nil
}

func (c *controller) NetworkByID(id string) (Network, error) {
	if id == "" {
		return nil, ErrInvalidID(id)
	}
	c.Lock()
	defer c.Unlock()
	if n, ok := c.networks[types.UUID(id)]; ok {
		return n, nil
	}
	return nil, ErrNoSuchNetwork(id)
}

func (c *controller) sandboxAdd(key string, create bool) (sandbox.Sandbox, error) {
	c.Lock()
	defer c.Unlock()

	sData, ok := c.sandboxes[key]
	if !ok {
		sb, err := sandbox.NewSandbox(key, create)
		if err != nil {
			return nil, err
		}

		sData = &sandboxData{sandbox: sb, refCnt: 1}
		c.sandboxes[key] = sData
		return sData.sandbox, nil
	}

	sData.refCnt++
	return sData.sandbox, nil
}

func (c *controller) sandboxRm(key string) {
	c.Lock()
	defer c.Unlock()

	sData := c.sandboxes[key]
	sData.refCnt--

	if sData.refCnt == 0 {
		sData.sandbox.Destroy()
		delete(c.sandboxes, key)
	}
}

func (c *controller) sandboxGet(key string) sandbox.Sandbox {
	c.Lock()
	defer c.Unlock()

	sData, ok := c.sandboxes[key]
	if !ok {
		return nil
	}

	return sData.sandbox
}

func (c *controller) loadDriver(networkType string) (driverapi.Driver, error) {
	// Plugins pkg performs lazy loading of plugins that acts as remote drivers.
	// As per the design, this Get call will result in remote driver discovery if there is a corresponding plugin available.
	_, err := plugins.Get(networkType, driverapi.NetworkPluginEndpointType)
	if err != nil {
		if err == plugins.ErrNotFound {
			return nil, types.NotFoundErrorf(err.Error())
		}
		return nil, err
	}
	c.Lock()
	defer c.Unlock()
	d, ok := c.drivers[networkType]
	if !ok {
		return nil, ErrInvalidNetworkDriver(networkType)
	}
	return d, nil
}
