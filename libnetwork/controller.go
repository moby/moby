/*
Package libnetwork provides the basic functionality and extension points to
create network namespaces and allocate interfaces for containers to use.

        // Create a new controller instance
        controller, _err := libnetwork.New("/etc/default/libnetwork.toml")

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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/hostdiscovery"
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
func New(configFile string) (NetworkController, error) {
	c := &controller{
		networks:  networkTable{},
		sandboxes: sandboxTable{},
		drivers:   driverTable{}}
	if err := initDrivers(c); err != nil {
		return nil, err
	}

	if err := c.initConfig(configFile); err == nil {
		if err := c.initDataStore(); err != nil {
			// Failing to initalize datastore is a bad situation to be in.
			// But it cannot fail creating the Controller
			log.Warnf("Failed to Initialize Datastore due to %v. Operating in non-clustered mode", err)
		}
		if err := c.initDiscovery(); err != nil {
			// Failing to initalize discovery is a bad situation to be in.
			// But it cannot fail creating the Controller
			log.Warnf("Failed to Initialize Discovery : %v", err)
		}
	} else {
		// Missing Configuration file is not a failure scenario
		// But without that, datastore cannot be initialized.
		log.Debugf("Unable to Parse LibNetwork Config file : %v", err)
	}

	return c, nil
}

const (
	cfgFileEnv     = "LIBNETWORK_CFG"
	defaultCfgFile = "/etc/default/libnetwork.toml"
)

func (c *controller) initConfig(configFile string) error {
	cfgFile := configFile
	if strings.Trim(cfgFile, " ") == "" {
		cfgFile = os.Getenv(cfgFileEnv)
		if strings.Trim(cfgFile, " ") == "" {
			cfgFile = defaultCfgFile
		}
	}
	cfg, err := config.ParseConfig(cfgFile)
	if err != nil {
		return ErrInvalidConfigFile(cfgFile)
	}
	c.Lock()
	c.cfg = cfg
	c.Unlock()
	return nil
}

func (c *controller) initDataStore() error {
	if c.cfg == nil {
		return fmt.Errorf("datastore initialization requires a valid configuration")
	}

	store, err := datastore.NewDataStore(&c.cfg.Datastore)
	if err != nil {
		return err
	}
	c.Lock()
	c.store = store
	c.Unlock()
	go c.watchNewNetworks()

	return nil
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
	if err := c.addNetworkToStore(network); err != nil {
		return nil, err
	}
	// Create the network
	if err := d.CreateNetwork(network.id, network.generic); err != nil {
		return nil, err
	}

	// Store the network handler in controller
	c.Lock()
	c.networks[network.id] = network
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

func (c *controller) addNetworkToStore(n *network) error {
	if isReservedNetwork(n.Name()) {
		return nil
	}
	c.Lock()
	cs := c.store
	c.Unlock()
	if cs == nil {
		log.Debugf("datastore not initialized. Network %s is not added to the store", n.Name())
		return nil
	}
	return cs.PutObjectAtomic(n)
}

func (c *controller) watchNewNetworks() {
	c.Lock()
	cs := c.store
	c.Unlock()

	cs.KVStore().WatchRange(datastore.Key(datastore.NetworkKeyPrefix), "", 0, func(kvi []store.KVEntry) {
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
			} else if ok {
				// Received an update for an existing network object
				log.Debugf("Skipping network update for %s (%s)", n.name, n.id)
				continue
			}

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
