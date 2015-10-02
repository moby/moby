/*
Package libnetwork provides the basic functionality and extension points to
create network namespaces and allocate interfaces for containers to use.

	networkType := "bridge"

	// Create a new controller instance
	driverOptions := options.Generic{}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = driverOptions
	controller, err := libnetwork.New(config.OptionDriverConfig(networkType, genericOption))
	if err != nil {
		return
	}

	// Create a network for containers to join.
	// NewNetwork accepts Variadic optional arguments that libnetwork and Drivers can make use of
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

	// Create the sandbox for the container.
	// NewSandbox accepts Variadic optional arguments which libnetwork can use.
	sbx, err := controller.NewSandbox("container1",
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"))

	// A sandbox can join the endpoint via the join api.
	err = ep.Join(sbx)
	if err != nil {
		return
	}
*/
package libnetwork

import (
	"container/heap"
	"fmt"
	"net"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/discovery"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/hostdiscovery"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/types"
)

// NetworkController provides the interface for controller instance which manages
// networks.
type NetworkController interface {
	// ID provides an unique identity for the controller
	ID() string

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

	// NewSandbox cretes a new network sandbox for the passed container id
	NewSandbox(containerID string, options ...SandboxOption) (Sandbox, error)

	// Sandboxes returns the list of Sandbox(s) managed by this controller.
	Sandboxes() []Sandbox

	// WlakSandboxes uses the provided function to walk the Sandbox(s) managed by this controller.
	WalkSandboxes(walker SandboxWalker)

	// SandboxByID returns the Sandbox which has the passed id. If not found, a types.NotFoundError is returned.
	SandboxByID(id string) (Sandbox, error)

	// Stop network controller
	Stop()
}

// NetworkWalker is a client provided function which will be used to walk the Networks.
// When the function returns true, the walk will stop.
type NetworkWalker func(nw Network) bool

// SandboxWalker is a client provided function which will be used to walk the Sandboxes.
// When the function returns true, the walk will stop.
type SandboxWalker func(sb Sandbox) bool

type driverData struct {
	driver     driverapi.Driver
	capability driverapi.Capability
}

type driverTable map[string]*driverData
type networkTable map[string]*network
type endpointTable map[string]*endpoint
type sandboxTable map[string]*sandbox

type controller struct {
	id                      string
	networks                networkTable
	drivers                 driverTable
	sandboxes               sandboxTable
	cfg                     *config.Config
	globalStore, localStore datastore.DataStore
	discovery               hostdiscovery.HostDiscovery
	extKeyListener          net.Listener
	sync.Mutex
}

// New creates a new instance of network controller.
func New(cfgOptions ...config.Option) (NetworkController, error) {
	var cfg *config.Config
	if len(cfgOptions) > 0 {
		cfg = &config.Config{
			Daemon: config.DaemonCfg{
				DriverCfg: make(map[string]interface{}),
			},
		}
		cfg.ProcessOptions(cfgOptions...)
	}
	c := &controller{
		id:        stringid.GenerateRandomID(),
		cfg:       cfg,
		networks:  networkTable{},
		sandboxes: sandboxTable{},
		drivers:   driverTable{}}
	if err := initDrivers(c); err != nil {
		return nil, err
	}

	if cfg != nil {
		if err := c.initGlobalStore(); err != nil {
			// Failing to initalize datastore is a bad situation to be in.
			// But it cannot fail creating the Controller
			log.Debugf("Failed to Initialize Datastore due to %v. Operating in non-clustered mode", err)
		}
		if err := c.initDiscovery(cfg.Cluster.Watcher); err != nil {
			// Failing to initalize discovery is a bad situation to be in.
			// But it cannot fail creating the Controller
			log.Debugf("Failed to Initialize Discovery : %v", err)
		}
		if err := c.initLocalStore(); err != nil {
			log.Debugf("Failed to Initialize LocalDatastore due to %v.", err)
		}
	}

	if err := c.startExternalKeyListener(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *controller) ID() string {
	return c.id
}

func (c *controller) validateHostDiscoveryConfig() bool {
	if c.cfg == nil || c.cfg.Cluster.Discovery == "" || c.cfg.Cluster.Address == "" {
		return false
	}
	return true
}

func (c *controller) initDiscovery(watcher discovery.Watcher) error {
	if c.cfg == nil {
		return fmt.Errorf("discovery initialization requires a valid configuration")
	}

	c.discovery = hostdiscovery.NewHostDiscovery(watcher)
	return c.discovery.Watch(c.hostJoinCallback, c.hostLeaveCallback)
}

func (c *controller) hostJoinCallback(nodes []net.IP) {
	c.processNodeDiscovery(nodes, true)
}

func (c *controller) hostLeaveCallback(nodes []net.IP) {
	c.processNodeDiscovery(nodes, false)
}

func (c *controller) processNodeDiscovery(nodes []net.IP, add bool) {
	c.Lock()
	drivers := []*driverData{}
	for _, d := range c.drivers {
		drivers = append(drivers, d)
	}
	c.Unlock()

	for _, d := range drivers {
		c.pushNodeDiscovery(d, nodes, add)
	}
}

func (c *controller) pushNodeDiscovery(d *driverData, nodes []net.IP, add bool) {
	var self net.IP
	if c.cfg != nil {
		addr := strings.Split(c.cfg.Cluster.Address, ":")
		self = net.ParseIP(addr[0])
	}
	if d == nil || d.capability.DataScope != datastore.GlobalScope || nodes == nil {
		return
	}
	for _, node := range nodes {
		nodeData := driverapi.NodeDiscoveryData{Address: node.String(), Self: node.Equal(self)}
		var err error
		if add {
			err = d.driver.DiscoverNew(driverapi.NodeDiscovery, nodeData)
		} else {
			err = d.driver.DiscoverDelete(driverapi.NodeDiscovery, nodeData)
		}
		if err != nil {
			log.Debugf("discovery notification error : %v", err)
		}
	}
}

func (c *controller) Config() config.Config {
	c.Lock()
	defer c.Unlock()
	if c.cfg == nil {
		return config.Config{}
	}
	return *c.cfg
}

func (c *controller) RegisterDriver(networkType string, driver driverapi.Driver, capability driverapi.Capability) error {
	if !config.IsValidName(networkType) {
		return ErrInvalidName(networkType)
	}

	c.Lock()
	if _, ok := c.drivers[networkType]; ok {
		c.Unlock()
		return driverapi.ErrActiveRegistration(networkType)
	}
	dData := &driverData{driver, capability}
	c.drivers[networkType] = dData
	hd := c.discovery
	c.Unlock()

	if hd != nil {
		c.pushNodeDiscovery(dData, hd.Fetch(), true)
	}

	return nil
}

// NewNetwork creates a new network of the specified network type. The options
// are network specific and modeled in a generic way.
func (c *controller) NewNetwork(networkType, name string, options ...NetworkOption) (Network, error) {
	if !config.IsValidName(name) {
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
		id:          stringid.GenerateRandomID(),
		ctrlr:       c,
		endpoints:   endpointTable{},
		persist:     true,
	}

	network.processOptions(options...)

	if err := c.addNetwork(network); err != nil {
		return nil, err
	}

	if err := c.updateToStore(network); err != nil {
		log.Warnf("couldnt create network %s: %v", network.name, err)
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
	n.svcRecords = svcMap{}
	n.driver = dd.driver
	n.dataScope = dd.capability.DataScope
	d := n.driver
	n.Unlock()

	// Create the network
	if err := d.CreateNetwork(n.id, n.generic); err != nil {
		return err
	}
	if n.isGlobalScoped() {
		if err := n.watchEndpoints(); err != nil {
			return err
		}
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
	if n, ok := c.networks[id]; ok {
		return n, nil
	}
	return nil, ErrNoSuchNetwork(id)
}

// NewSandbox creates a new sandbox for the passed container id
func (c *controller) NewSandbox(containerID string, options ...SandboxOption) (Sandbox, error) {
	var err error

	if containerID == "" {
		return nil, types.BadRequestErrorf("invalid container ID")
	}

	var existing Sandbox
	look := SandboxContainerWalker(&existing, containerID)
	c.WalkSandboxes(look)
	if existing != nil {
		return nil, types.BadRequestErrorf("container %s is already present: %v", containerID, existing)
	}

	// Create sandbox and process options first. Key generation depends on an option
	sb := &sandbox{
		id:          stringid.GenerateRandomID(),
		containerID: containerID,
		endpoints:   epHeap{},
		epPriority:  map[string]int{},
		config:      containerConfig{},
		controller:  c,
	}
	// This sandbox may be using an existing osl sandbox, sharing it with another sandbox
	var peerSb Sandbox
	c.WalkSandboxes(SandboxKeyWalker(&peerSb, sb.Key()))
	if peerSb != nil {
		sb.osSbox = peerSb.(*sandbox).osSbox
	}

	heap.Init(&sb.endpoints)

	sb.processOptions(options...)

	if err = sb.setupResolutionFiles(); err != nil {
		return nil, err
	}

	if sb.osSbox == nil && !sb.config.useExternalKey {
		if sb.osSbox, err = osl.NewSandbox(sb.Key(), !sb.config.useDefaultSandBox); err != nil {
			return nil, fmt.Errorf("failed to create new osl sandbox: %v", err)
		}
	}

	c.Lock()
	c.sandboxes[sb.id] = sb
	c.Unlock()

	return sb, nil
}

func (c *controller) Sandboxes() []Sandbox {
	c.Lock()
	defer c.Unlock()

	list := make([]Sandbox, 0, len(c.sandboxes))
	for _, s := range c.sandboxes {
		list = append(list, s)
	}

	return list
}

func (c *controller) WalkSandboxes(walker SandboxWalker) {
	for _, sb := range c.Sandboxes() {
		if walker(sb) {
			return
		}
	}
}

func (c *controller) SandboxByID(id string) (Sandbox, error) {
	if id == "" {
		return nil, ErrInvalidID(id)
	}
	c.Lock()
	s, ok := c.sandboxes[id]
	c.Unlock()
	if !ok {
		return nil, types.NotFoundErrorf("sandbox %s not found", id)
	}
	return s, nil
}

// SandboxContainerWalker returns a Sandbox Walker function which looks for an existing Sandbox with the passed containerID
func SandboxContainerWalker(out *Sandbox, containerID string) SandboxWalker {
	return func(sb Sandbox) bool {
		if sb.ContainerID() == containerID {
			*out = sb
			return true
		}
		return false
	}
}

// SandboxKeyWalker returns a Sandbox Walker function which looks for an existing Sandbox with the passed key
func SandboxKeyWalker(out *Sandbox, key string) SandboxWalker {
	return func(sb Sandbox) bool {
		if sb.Key() == key {
			*out = sb
			return true
		}
		return false
	}
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

func (c *controller) getDriver(networkType string) (*driverData, error) {
	c.Lock()
	defer c.Unlock()
	dd, ok := c.drivers[networkType]
	if !ok {
		return nil, types.NotFoundErrorf("driver %s not found", networkType)
	}
	return dd, nil
}

func (c *controller) Stop() {
	if c.localStore != nil {
		c.localStore.KVStore().Close()
	}
	c.stopExternalKeyListener()
	osl.GC()
}
