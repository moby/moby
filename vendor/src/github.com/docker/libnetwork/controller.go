/*
Package libnetwork provides the basic functionality and extension points to
create network namespaces and allocate interfaces for containers to use.

        // Create a new controller instance
        controller, _err := libnetwork.New(nil)

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
        // api.
        // Join acceps Variadic arguments which will be made use of by libnetwork and Drivers
        err = ep.Join("container1",
                libnetwork.JoinOptionHostname("test"),
                libnetwork.JoinOptionDomainname("docker.io"))
        if err != nil {
                return
        }
*/
package libnetwork

import (
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/hostdiscovery"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

// NetworkController provides the interface for controller instance which manages
// networks.
type NetworkController interface {
	// ConfigureNetworkDriver applies the passed options to the driver instance for the specified network type
	ConfigureNetworkDriver(networkType string, options map[string]interface{}) error

	// Config method returns the bootup configuration for the controller
	Config() config.Config

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

	// GC triggers immediate garbage collection of resources which are garbage collected.
	GC()
}

// NetworkWalker is a client provided function which will be used to walk the Networks.
// When the function returns true, the walk will stop.
type NetworkWalker func(nw Network) bool

type driverData struct {
	driver     driverapi.Driver
	capability driverapi.Capability
}

type driverTable map[string]*driverData
type networkTable map[types.UUID]*network
type endpointTable map[types.UUID]*endpoint
type sandboxTable map[string]*sandboxData

type controller struct {
	networks  networkTable
	drivers   driverTable
	sandboxes sandboxTable
	cfg       *config.Config
	store     datastore.DataStore
	sync.Mutex
}

// New creates a new instance of network controller.
func New(cfgOptions ...config.Option) (NetworkController, error) {
	var cfg *config.Config
	if len(cfgOptions) > 0 {
		cfg = &config.Config{}
		cfg.ProcessOptions(cfgOptions...)
	}
	c := &controller{
		cfg:       cfg,
		networks:  networkTable{},
		sandboxes: sandboxTable{},
		drivers:   driverTable{}}
	if err := initDrivers(c); err != nil {
		return nil, err
	}

	if cfg != nil {
		if err := c.initDataStore(); err != nil {
			// Failing to initalize datastore is a bad situation to be in.
			// But it cannot fail creating the Controller
			log.Debugf("Failed to Initialize Datastore due to %v. Operating in non-clustered mode", err)
		}
		if err := c.initDiscovery(); err != nil {
			// Failing to initalize discovery is a bad situation to be in.
			// But it cannot fail creating the Controller
			log.Debugf("Failed to Initialize Discovery : %v", err)
		}
	}

	return c, nil
}

func (c *controller) validateHostDiscoveryConfig() bool {
	if c.cfg == nil || c.cfg.Cluster.Discovery == "" || c.cfg.Cluster.Address == "" {
		return false
	}
	return true
}

func (c *controller) initDiscovery() error {
	if c.cfg == nil {
		return fmt.Errorf("discovery initialization requires a valid configuration")
	}

	hostDiscovery := hostdiscovery.NewHostDiscovery()
	return hostDiscovery.StartDiscovery(&c.cfg.Cluster, c.hostJoinCallback, c.hostLeaveCallback)
}

func (c *controller) hostJoinCallback(hosts []net.IP) {
}

func (c *controller) hostLeaveCallback(hosts []net.IP) {
}

func (c *controller) Config() config.Config {
	c.Lock()
	defer c.Unlock()
	return *c.cfg
}

func (c *controller) ConfigureNetworkDriver(networkType string, options map[string]interface{}) error {
	c.Lock()
	dd, ok := c.drivers[networkType]
	c.Unlock()
	if !ok {
		return NetworkTypeError(networkType)
	}
	return dd.driver.Config(options)
}

func (c *controller) RegisterDriver(networkType string, driver driverapi.Driver, capability driverapi.Capability) error {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.drivers[networkType]; ok {
		return driverapi.ErrActiveRegistration(networkType)
	}
	c.drivers[networkType] = &driverData{driver, capability}
	return nil
}

// NewNetwork creates a new network of the specified network type. The options
// are network specific and modeled in a generic way.
func (c *controller) NewNetwork(networkType, name string, options ...NetworkOption) (Network, error) {
	if name == "" {
		return nil, ErrInvalidName(name)
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
		name:        name,
		networkType: networkType,
		id:          types.UUID(stringid.GenerateRandomID()),
		ctrlr:       c,
		endpoints:   endpointTable{},
	}

	network.processOptions(options...)

	if err := c.addNetwork(network); err != nil {
		return nil, err
	}

	if err := c.updateNetworkToStore(network); err != nil {
		if e := network.Delete(); e != nil {
			log.Warnf("couldnt cleanup network %s: %v", network.name, err)
		}
		return nil, err
	}

	return network, nil
}

func (c *controller) addNetwork(n *network) error {

	c.Lock()
	// Check if a driver for the specified network type is available
	dd, ok := c.drivers[n.networkType]
	c.Unlock()

	if !ok {
		var err error
		dd, err = c.loadDriver(n.networkType)
		if err != nil {
			return err
		}
	}

	n.Lock()
	n.driver = dd.driver
	d := n.driver
	n.Unlock()

	// Create the network
	if err := d.CreateNetwork(n.id, n.generic); err != nil {
		return err
	}
	c.Lock()
	c.networks[n.id] = n
	c.Unlock()

	return nil
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

func (c *controller) loadDriver(networkType string) (*driverData, error) {
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
	dd, ok := c.drivers[networkType]
	if !ok {
		return nil, ErrInvalidNetworkDriver(networkType)
	}
	return dd, nil
}

func (c *controller) isDriverGlobalScoped(networkType string) (bool, error) {
	c.Lock()
	dd, ok := c.drivers[networkType]
	c.Unlock()
	if !ok {
		return false, types.NotFoundErrorf("driver not found for %s", networkType)
	}
	if dd.capability.Scope == driverapi.GlobalScope {
		return true, nil
	}
	return false, nil
}

func (c *controller) GC() {
	sandbox.GC()
}
