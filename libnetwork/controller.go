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
	network, err := controller.NewNetwork(networkType, "network1", "")
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
		libnetwork.OptionDomainname("example.com"))

	// A sandbox can join the endpoint via the join api.
	err = ep.Join(sbx)
	if err != nil {
		return
	}
*/
package libnetwork

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/libnetwork/cluster"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/diagnostic"
	"github.com/docker/docker/libnetwork/discoverapi"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drvregistry"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/moby/locker"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NetworkWalker is a client provided function which will be used to walk the Networks.
// When the function returns true, the walk will stop.
type NetworkWalker func(nw Network) bool

// SandboxWalker is a client provided function which will be used to walk the Sandboxes.
// When the function returns true, the walk will stop.
type SandboxWalker func(sb *Sandbox) bool

type sandboxTable map[string]*Sandbox

// Controller manages networks.
type Controller struct {
	id               string
	drvRegistry      *drvregistry.DrvRegistry
	sandboxes        sandboxTable
	cfg              *config.Config
	stores           []datastore.DataStore
	extKeyListener   net.Listener
	watchCh          chan *Endpoint
	unWatchCh        chan *Endpoint
	svcRecords       map[string]svcInfo
	nmap             map[string]*netWatch
	serviceBindings  map[serviceKey]*service
	defOsSbox        osl.Sandbox
	ingressSandbox   *Sandbox
	sboxOnce         sync.Once
	agent            *agent
	networkLocker    *locker.Locker
	agentInitDone    chan struct{}
	agentStopDone    chan struct{}
	keys             []*types.EncryptionKey
	DiagnosticServer *diagnostic.Server
	mu               sync.Mutex
}

type initializer struct {
	fn    drvregistry.InitFunc
	ntype string
}

// New creates a new instance of network controller.
func New(cfgOptions ...config.Option) (*Controller, error) {
	c := &Controller{
		id:               stringid.GenerateRandomID(),
		cfg:              config.New(cfgOptions...),
		sandboxes:        sandboxTable{},
		svcRecords:       make(map[string]svcInfo),
		serviceBindings:  make(map[serviceKey]*service),
		agentInitDone:    make(chan struct{}),
		networkLocker:    locker.New(),
		DiagnosticServer: diagnostic.New(),
	}
	c.DiagnosticServer.Init()

	if err := c.initStores(); err != nil {
		return nil, err
	}

	drvRegistry, err := drvregistry.New(c.getStore(datastore.LocalScope), c.getStore(datastore.GlobalScope), c.RegisterDriver, nil, c.cfg.PluginGetter)
	if err != nil {
		return nil, err
	}

	for _, i := range getInitializers() {
		var dcfg map[string]interface{}

		// External plugins don't need config passed through daemon. They can
		// bootstrap themselves
		if i.ntype != "remote" {
			dcfg = c.makeDriverConfig(i.ntype)
		}

		if err := drvRegistry.AddDriver(i.ntype, i.fn, dcfg); err != nil {
			return nil, err
		}
	}

	if err = initIPAMDrivers(drvRegistry, nil, c.getStore(datastore.GlobalScope), c.cfg.DefaultAddressPool); err != nil {
		return nil, err
	}

	c.drvRegistry = drvRegistry

	c.WalkNetworks(populateSpecial)

	// Reserve pools first before doing cleanup. Otherwise the
	// cleanups of endpoint/network and sandbox below will
	// generate many unnecessary warnings
	c.reservePools()

	// Cleanup resources
	c.sandboxCleanup(c.cfg.ActiveSandboxes)
	c.cleanupLocalEndpoints()
	c.networkCleanup()

	if err := c.startExternalKeyListener(); err != nil {
		return nil, err
	}

	setupArrangeUserFilterRule(c)
	return c, nil
}

// SetClusterProvider sets the cluster provider.
func (c *Controller) SetClusterProvider(provider cluster.Provider) {
	var sameProvider bool
	c.mu.Lock()
	// Avoids to spawn multiple goroutine for the same cluster provider
	if c.cfg.ClusterProvider == provider {
		// If the cluster provider is already set, there is already a go routine spawned
		// that is listening for events, so nothing to do here
		sameProvider = true
	} else {
		c.cfg.ClusterProvider = provider
	}
	c.mu.Unlock()

	if provider == nil || sameProvider {
		return
	}
	// We don't want to spawn a new go routine if the previous one did not exit yet
	c.AgentStopWait()
	go c.clusterAgentInit()
}

// SetKeys configures the encryption key for gossip and overlay data path.
func (c *Controller) SetKeys(keys []*types.EncryptionKey) error {
	// libnetwork side of agent depends on the keys. On the first receipt of
	// keys setup the agent. For subsequent key set handle the key change
	subsysKeys := make(map[string]int)
	for _, key := range keys {
		if key.Subsystem != subsysGossip &&
			key.Subsystem != subsysIPSec {
			return fmt.Errorf("key received for unrecognized subsystem")
		}
		subsysKeys[key.Subsystem]++
	}
	for s, count := range subsysKeys {
		if count != keyringSize {
			return fmt.Errorf("incorrect number of keys for subsystem %v", s)
		}
	}

	if c.getAgent() == nil {
		c.mu.Lock()
		c.keys = keys
		c.mu.Unlock()
		return nil
	}
	return c.handleKeyChange(keys)
}

func (c *Controller) getAgent() *agent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.agent
}

func (c *Controller) clusterAgentInit() {
	clusterProvider := c.cfg.ClusterProvider
	var keysAvailable bool
	for {
		eventType := <-clusterProvider.ListenClusterEvents()
		// The events: EventSocketChange, EventNodeReady and EventNetworkKeysAvailable are not ordered
		// when all the condition for the agent initialization are met then proceed with it
		switch eventType {
		case cluster.EventNetworkKeysAvailable:
			// Validates that the keys are actually available before starting the initialization
			// This will handle old spurious messages left on the channel
			c.mu.Lock()
			keysAvailable = c.keys != nil
			c.mu.Unlock()
			fallthrough
		case cluster.EventSocketChange, cluster.EventNodeReady:
			if keysAvailable && !c.isDistributedControl() {
				c.agentOperationStart()
				if err := c.agentSetup(clusterProvider); err != nil {
					c.agentStopComplete()
				} else {
					c.agentInitComplete()
				}
			}
		case cluster.EventNodeLeave:
			c.agentOperationStart()
			c.mu.Lock()
			c.keys = nil
			c.mu.Unlock()

			// We are leaving the cluster. Make sure we
			// close the gossip so that we stop all
			// incoming gossip updates before cleaning up
			// any remaining service bindings. But before
			// deleting the networks since the networks
			// should still be present when cleaning up
			// service bindings
			c.agentClose()
			c.cleanupServiceDiscovery("")
			c.cleanupServiceBindings("")

			c.agentStopComplete()

			return
		}
	}
}

// AgentInitWait waits for agent initialization to be completed in the controller.
func (c *Controller) AgentInitWait() {
	c.mu.Lock()
	agentInitDone := c.agentInitDone
	c.mu.Unlock()

	if agentInitDone != nil {
		<-agentInitDone
	}
}

// AgentStopWait waits for the Agent stop to be completed in the controller.
func (c *Controller) AgentStopWait() {
	c.mu.Lock()
	agentStopDone := c.agentStopDone
	c.mu.Unlock()
	if agentStopDone != nil {
		<-agentStopDone
	}
}

// agentOperationStart marks the start of an Agent Init or Agent Stop
func (c *Controller) agentOperationStart() {
	c.mu.Lock()
	if c.agentInitDone == nil {
		c.agentInitDone = make(chan struct{})
	}
	if c.agentStopDone == nil {
		c.agentStopDone = make(chan struct{})
	}
	c.mu.Unlock()
}

// agentInitComplete notifies the successful completion of the Agent initialization
func (c *Controller) agentInitComplete() {
	c.mu.Lock()
	if c.agentInitDone != nil {
		close(c.agentInitDone)
		c.agentInitDone = nil
	}
	c.mu.Unlock()
}

// agentStopComplete notifies the successful completion of the Agent stop
func (c *Controller) agentStopComplete() {
	c.mu.Lock()
	if c.agentStopDone != nil {
		close(c.agentStopDone)
		c.agentStopDone = nil
	}
	c.mu.Unlock()
}

func (c *Controller) makeDriverConfig(ntype string) map[string]interface{} {
	if c.cfg == nil {
		return nil
	}

	cfg := map[string]interface{}{}
	for _, label := range c.cfg.Labels {
		key, val, _ := strings.Cut(label, "=")
		if !strings.HasPrefix(key, netlabel.DriverPrefix+"."+ntype) {
			continue
		}

		cfg[key] = val
	}

	drvCfg, ok := c.cfg.DriverCfg[ntype]
	if ok {
		for k, v := range drvCfg.(map[string]interface{}) {
			cfg[k] = v
		}
	}

	for k, v := range c.cfg.Scopes {
		if !v.IsValid() {
			continue
		}
		cfg[netlabel.MakeKVClient(k)] = discoverapi.DatastoreConfigData{
			Scope:    k,
			Provider: v.Client.Provider,
			Address:  v.Client.Address,
			Config:   v.Client.Config,
		}
	}

	return cfg
}

var procReloadConfig = make(chan (bool), 1)

// ReloadConfiguration updates the controller configuration.
func (c *Controller) ReloadConfiguration(cfgOptions ...config.Option) error {
	procReloadConfig <- true
	defer func() { <-procReloadConfig }()

	// For now we accept the configuration reload only as a mean to provide a global store config after boot.
	// Refuse the configuration if it alters an existing datastore client configuration.
	update := false
	cfg := config.New(cfgOptions...)

	for s := range c.cfg.Scopes {
		if _, ok := cfg.Scopes[s]; !ok {
			return types.ForbiddenErrorf("cannot accept new configuration because it removes an existing datastore client")
		}
	}
	for s, nSCfg := range cfg.Scopes {
		if eSCfg, ok := c.cfg.Scopes[s]; ok {
			if eSCfg.Client.Provider != nSCfg.Client.Provider ||
				eSCfg.Client.Address != nSCfg.Client.Address {
				return types.ForbiddenErrorf("cannot accept new configuration because it modifies an existing datastore client")
			}
		} else {
			if err := c.initScopedStore(s, nSCfg); err != nil {
				return err
			}
			update = true
		}
	}
	if !update {
		return nil
	}

	c.mu.Lock()
	c.cfg = cfg
	c.mu.Unlock()

	var dsConfig *discoverapi.DatastoreConfigData
	for scope, sCfg := range cfg.Scopes {
		if scope == datastore.LocalScope || !sCfg.IsValid() {
			continue
		}
		dsConfig = &discoverapi.DatastoreConfigData{
			Scope:    scope,
			Provider: sCfg.Client.Provider,
			Address:  sCfg.Client.Address,
			Config:   sCfg.Client.Config,
		}
		break
	}
	if dsConfig == nil {
		return nil
	}

	c.drvRegistry.WalkIPAMs(func(name string, driver ipamapi.Ipam, cap *ipamapi.Capability) bool {
		err := driver.DiscoverNew(discoverapi.DatastoreConfig, *dsConfig)
		if err != nil {
			logrus.Errorf("Failed to set datastore in driver %s: %v", name, err)
		}
		return false
	})

	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		err := driver.DiscoverNew(discoverapi.DatastoreConfig, *dsConfig)
		if err != nil {
			logrus.Errorf("Failed to set datastore in driver %s: %v", name, err)
		}
		return false
	})
	return nil
}

// ID returns the controller's unique identity.
func (c *Controller) ID() string {
	return c.id
}

// BuiltinDrivers returns the list of builtin network drivers.
func (c *Controller) BuiltinDrivers() []string {
	drivers := []string{}
	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		if driver.IsBuiltIn() {
			drivers = append(drivers, name)
		}
		return false
	})
	return drivers
}

// BuiltinIPAMDrivers returns the list of builtin ipam drivers.
func (c *Controller) BuiltinIPAMDrivers() []string {
	drivers := []string{}
	c.drvRegistry.WalkIPAMs(func(name string, driver ipamapi.Ipam, cap *ipamapi.Capability) bool {
		if driver.IsBuiltIn() {
			drivers = append(drivers, name)
		}
		return false
	})
	return drivers
}

func (c *Controller) processNodeDiscovery(nodes []net.IP, add bool) {
	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		c.pushNodeDiscovery(driver, capability, nodes, add)
		return false
	})
}

func (c *Controller) pushNodeDiscovery(d driverapi.Driver, cap driverapi.Capability, nodes []net.IP, add bool) {
	var self net.IP
	// try swarm-mode config
	if agent := c.getAgent(); agent != nil {
		self = net.ParseIP(agent.advertiseAddr)
	}

	if d == nil || cap.ConnectivityScope != datastore.GlobalScope || nodes == nil {
		return
	}

	for _, node := range nodes {
		nodeData := discoverapi.NodeDiscoveryData{Address: node.String(), Self: node.Equal(self)}
		var err error
		if add {
			err = d.DiscoverNew(discoverapi.NodeDiscovery, nodeData)
		} else {
			err = d.DiscoverDelete(discoverapi.NodeDiscovery, nodeData)
		}
		if err != nil {
			logrus.Debugf("discovery notification error: %v", err)
		}
	}
}

// Config returns the bootup configuration for the controller.
func (c *Controller) Config() config.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg == nil {
		return config.Config{}
	}
	return *c.cfg
}

func (c *Controller) isManager() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg == nil || c.cfg.ClusterProvider == nil {
		return false
	}
	return c.cfg.ClusterProvider.IsManager()
}

func (c *Controller) isAgent() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg == nil || c.cfg.ClusterProvider == nil {
		return false
	}
	return c.cfg.ClusterProvider.IsAgent()
}

func (c *Controller) isDistributedControl() bool {
	return !c.isManager() && !c.isAgent()
}

func (c *Controller) GetPluginGetter() plugingetter.PluginGetter {
	return c.drvRegistry.GetPluginGetter()
}

func (c *Controller) RegisterDriver(networkType string, driver driverapi.Driver, capability driverapi.Capability) error {
	c.agentDriverNotify(driver)
	return nil
}

// XXX  This should be made driver agnostic.  See comment below.
const overlayDSROptionString = "dsr"

// NewNetwork creates a new network of the specified network type. The options
// are network specific and modeled in a generic way.
func (c *Controller) NewNetwork(networkType, name string, id string, options ...NetworkOption) (Network, error) {
	var (
		caps           *driverapi.Capability
		err            error
		t              *network
		skipCfgEpCount bool
	)

	if id != "" {
		c.networkLocker.Lock(id)
		defer c.networkLocker.Unlock(id) //nolint:errcheck

		if _, err = c.NetworkByID(id); err == nil {
			return nil, NetworkNameError(id)
		}
	}

	if !config.IsValidName(name) {
		return nil, ErrInvalidName(name)
	}

	if id == "" {
		id = stringid.GenerateRandomID()
	}

	defaultIpam := defaultIpamForNetworkType(networkType)
	// Construct the network object
	nw := &network{
		name:             name,
		networkType:      networkType,
		generic:          map[string]interface{}{netlabel.GenericData: make(map[string]string)},
		ipamType:         defaultIpam,
		id:               id,
		created:          time.Now(),
		ctrlr:            c,
		persist:          true,
		drvOnce:          &sync.Once{},
		loadBalancerMode: loadBalancerModeDefault,
	}

	nw.processOptions(options...)
	if err = nw.validateConfiguration(); err != nil {
		return nil, err
	}

	// Reset network types, force local scope and skip allocation and
	// plumbing for configuration networks. Reset of the config-only
	// network drivers is needed so that this special network is not
	// usable by old engine versions.
	if nw.configOnly {
		nw.scope = datastore.LocalScope
		nw.networkType = "null"
		goto addToStore
	}

	_, caps, err = nw.resolveDriver(nw.networkType, true)
	if err != nil {
		return nil, err
	}

	if nw.scope == datastore.LocalScope && caps.DataScope == datastore.GlobalScope {
		return nil, types.ForbiddenErrorf("cannot downgrade network scope for %s networks", networkType)
	}
	if nw.ingress && caps.DataScope != datastore.GlobalScope {
		return nil, types.ForbiddenErrorf("Ingress network can only be global scope network")
	}

	// At this point the network scope is still unknown if not set by user
	if (caps.DataScope == datastore.GlobalScope || nw.scope == datastore.SwarmScope) &&
		!c.isDistributedControl() && !nw.dynamic {
		if c.isManager() {
			// For non-distributed controlled environment, globalscoped non-dynamic networks are redirected to Manager
			return nil, ManagerRedirectError(name)
		}
		return nil, types.ForbiddenErrorf("Cannot create a multi-host network from a worker node. Please create the network from a manager node.")
	}

	if nw.scope == datastore.SwarmScope && c.isDistributedControl() {
		return nil, types.ForbiddenErrorf("cannot create a swarm scoped network when swarm is not active")
	}

	// Make sure we have a driver available for this network type
	// before we allocate anything.
	if _, err := nw.driver(true); err != nil {
		return nil, err
	}

	// From this point on, we need the network specific configuration,
	// which may come from a configuration-only network
	if nw.configFrom != "" {
		t, err = c.getConfigNetwork(nw.configFrom)
		if err != nil {
			return nil, types.NotFoundErrorf("configuration network %q does not exist", nw.configFrom)
		}
		if err = t.applyConfigurationTo(nw); err != nil {
			return nil, types.InternalErrorf("Failed to apply configuration: %v", err)
		}
		nw.generic[netlabel.Internal] = nw.internal
		defer func() {
			if err == nil && !skipCfgEpCount {
				if err := t.getEpCnt().IncEndpointCnt(); err != nil {
					logrus.Warnf("Failed to update reference count for configuration network %q on creation of network %q: %v",
						t.Name(), nw.Name(), err)
				}
			}
		}()
	}

	err = nw.ipamAllocate()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			nw.ipamRelease()
		}
	}()

	err = c.addNetwork(nw)
	if err != nil {
		if _, ok := err.(types.MaskableError); ok { //nolint:gosimple
			// This error can be ignored and set this boolean
			// value to skip a refcount increment for configOnly networks
			skipCfgEpCount = true
		} else {
			return nil, err
		}
	}
	defer func() {
		if err != nil {
			if e := nw.deleteNetwork(); e != nil {
				logrus.Warnf("couldn't roll back driver network on network %s creation failure: %v", nw.name, err)
			}
		}
	}()

	// XXX If the driver type is "overlay" check the options for DSR
	// being set.  If so, set the network's load balancing mode to DSR.
	// This should really be done in a network option, but due to
	// time pressure to get this in without adding changes to moby,
	// swarm and CLI, it is being implemented as a driver-specific
	// option.  Unfortunately, drivers can't influence the core
	// "libnetwork.network" data type.  Hence we need this hack code
	// to implement in this manner.
	if gval, ok := nw.generic[netlabel.GenericData]; ok && nw.networkType == "overlay" {
		optMap := gval.(map[string]string)
		if _, ok := optMap[overlayDSROptionString]; ok {
			nw.loadBalancerMode = loadBalancerModeDSR
		}
	}

addToStore:
	// First store the endpoint count, then the network. To avoid to
	// end up with a datastore containing a network and not an epCnt,
	// in case of an ungraceful shutdown during this function call.
	epCnt := &endpointCnt{n: nw}
	if err = c.updateToStore(epCnt); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if e := c.deleteFromStore(epCnt); e != nil {
				logrus.Warnf("could not rollback from store, epCnt %v on failure (%v): %v", epCnt, err, e)
			}
		}
	}()

	nw.epCnt = epCnt
	if err = c.updateToStore(nw); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if e := c.deleteFromStore(nw); e != nil {
				logrus.Warnf("could not rollback from store, network %v on failure (%v): %v", nw, err, e)
			}
		}
	}()

	if nw.configOnly {
		return nw, nil
	}

	joinCluster(nw)
	defer func() {
		if err != nil {
			nw.cancelDriverWatches()
			if e := nw.leaveCluster(); e != nil {
				logrus.Warnf("Failed to leave agent cluster on network %s on failure (%v): %v", nw.name, err, e)
			}
		}
	}()

	if nw.hasLoadBalancerEndpoint() {
		if err = nw.createLoadBalancerSandbox(); err != nil {
			return nil, err
		}
	}

	if !c.isDistributedControl() {
		c.mu.Lock()
		arrangeIngressFilterRule()
		c.mu.Unlock()
	}
	arrangeUserFilterRule()

	return nw, nil
}

var joinCluster NetworkWalker = func(nw Network) bool {
	n := nw.(*network)
	if n.configOnly {
		return false
	}
	if err := n.joinCluster(); err != nil {
		logrus.Errorf("Failed to join network %s (%s) into agent cluster: %v", n.Name(), n.ID(), err)
	}
	n.addDriverWatches()
	return false
}

func (c *Controller) reservePools() {
	networks, err := c.getNetworksForScope(datastore.LocalScope)
	if err != nil {
		logrus.Warnf("Could not retrieve networks from local store during ipam allocation for existing networks: %v", err)
		return
	}

	for _, n := range networks {
		if n.configOnly {
			continue
		}
		if !doReplayPoolReserve(n) {
			continue
		}
		// Construct pseudo configs for the auto IP case
		autoIPv4 := (len(n.ipamV4Config) == 0 || (len(n.ipamV4Config) == 1 && n.ipamV4Config[0].PreferredPool == "")) && len(n.ipamV4Info) > 0
		autoIPv6 := (len(n.ipamV6Config) == 0 || (len(n.ipamV6Config) == 1 && n.ipamV6Config[0].PreferredPool == "")) && len(n.ipamV6Info) > 0
		if autoIPv4 {
			n.ipamV4Config = []*IpamConf{{PreferredPool: n.ipamV4Info[0].Pool.String()}}
		}
		if n.enableIPv6 && autoIPv6 {
			n.ipamV6Config = []*IpamConf{{PreferredPool: n.ipamV6Info[0].Pool.String()}}
		}
		// Account current network gateways
		for i, cfg := range n.ipamV4Config {
			if cfg.Gateway == "" && n.ipamV4Info[i].Gateway != nil {
				cfg.Gateway = n.ipamV4Info[i].Gateway.IP.String()
			}
		}
		if n.enableIPv6 {
			for i, cfg := range n.ipamV6Config {
				if cfg.Gateway == "" && n.ipamV6Info[i].Gateway != nil {
					cfg.Gateway = n.ipamV6Info[i].Gateway.IP.String()
				}
			}
		}
		// Reserve pools
		if err := n.ipamAllocate(); err != nil {
			logrus.Warnf("Failed to allocate ipam pool(s) for network %q (%s): %v", n.Name(), n.ID(), err)
		}
		// Reserve existing endpoints' addresses
		ipam, _, err := n.getController().getIPAMDriver(n.ipamType)
		if err != nil {
			logrus.Warnf("Failed to retrieve ipam driver for network %q (%s) during address reservation", n.Name(), n.ID())
			continue
		}
		epl, err := n.getEndpointsFromStore()
		if err != nil {
			logrus.Warnf("Failed to retrieve list of current endpoints on network %q (%s)", n.Name(), n.ID())
			continue
		}
		for _, ep := range epl {
			if ep.Iface() == nil {
				logrus.Warnf("endpoint interface is empty for %q (%s)", ep.Name(), ep.ID())
				continue
			}
			if err := ep.assignAddress(ipam, true, ep.Iface().AddressIPv6() != nil); err != nil {
				logrus.Warnf("Failed to reserve current address for endpoint %q (%s) on network %q (%s)",
					ep.Name(), ep.ID(), n.Name(), n.ID())
			}
		}
	}
}

func doReplayPoolReserve(n *network) bool {
	_, caps, err := n.getController().getIPAMDriver(n.ipamType)
	if err != nil {
		logrus.Warnf("Failed to retrieve ipam driver for network %q (%s): %v", n.Name(), n.ID(), err)
		return false
	}
	return caps.RequiresRequestReplay
}

func (c *Controller) addNetwork(n *network) error {
	d, err := n.driver(true)
	if err != nil {
		return err
	}

	// Create the network
	if err := d.CreateNetwork(n.id, n.generic, n, n.getIPData(4), n.getIPData(6)); err != nil {
		return err
	}

	n.startResolver()

	return nil
}

// Networks returns the list of Network(s) managed by this controller.
func (c *Controller) Networks() []Network {
	var list []Network

	for _, n := range c.getNetworksFromStore() {
		if n.inDelete {
			continue
		}
		list = append(list, n)
	}

	return list
}

// WalkNetworks uses the provided function to walk the Network(s) managed by this controller.
func (c *Controller) WalkNetworks(walker NetworkWalker) {
	for _, n := range c.Networks() {
		if walker(n) {
			return
		}
	}
}

// NetworkByName returns the Network which has the passed name.
// If not found, the error [ErrNoSuchNetwork] is returned.
func (c *Controller) NetworkByName(name string) (Network, error) {
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

// NetworkByID returns the Network which has the passed id.
// If not found, the error [ErrNoSuchNetwork] is returned.
func (c *Controller) NetworkByID(id string) (Network, error) {
	if id == "" {
		return nil, ErrInvalidID(id)
	}

	n, err := c.getNetworkFromStore(id)
	if err != nil {
		return nil, ErrNoSuchNetwork(id)
	}

	return n, nil
}

// NewSandbox creates a new sandbox for containerID.
func (c *Controller) NewSandbox(containerID string, options ...SandboxOption) (*Sandbox, error) {
	if containerID == "" {
		return nil, types.BadRequestErrorf("invalid container ID")
	}

	var sb *Sandbox
	c.mu.Lock()
	for _, s := range c.sandboxes {
		if s.containerID == containerID {
			// If not a stub, then we already have a complete sandbox.
			if !s.isStub {
				sbID := s.ID()
				c.mu.Unlock()
				return nil, types.ForbiddenErrorf("container %s is already present in sandbox %s", containerID, sbID)
			}

			// We already have a stub sandbox from the
			// store. Make use of it so that we don't lose
			// the endpoints from store but reset the
			// isStub flag.
			sb = s
			sb.isStub = false
			break
		}
	}
	c.mu.Unlock()

	sandboxID := stringid.GenerateRandomID()
	if runtime.GOOS == "windows" {
		sandboxID = containerID
	}

	// Create sandbox and process options first. Key generation depends on an option
	if sb == nil {
		sb = &Sandbox{
			id:                 sandboxID,
			containerID:        containerID,
			endpoints:          []*Endpoint{},
			epPriority:         map[string]int{},
			populatedEndpoints: map[string]struct{}{},
			config:             containerConfig{},
			controller:         c,
			extDNS:             []extDNSEntry{},
		}
	}

	sb.processOptions(options...)

	c.mu.Lock()
	if sb.ingress && c.ingressSandbox != nil {
		c.mu.Unlock()
		return nil, types.ForbiddenErrorf("ingress sandbox already present")
	}

	if sb.ingress {
		c.ingressSandbox = sb
		sb.config.hostsPath = filepath.Join(c.cfg.DataDir, "/network/files/hosts")
		sb.config.resolvConfPath = filepath.Join(c.cfg.DataDir, "/network/files/resolv.conf")
		sb.id = "ingress_sbox"
	} else if sb.loadBalancerNID != "" {
		sb.id = "lb_" + sb.loadBalancerNID
	}
	c.mu.Unlock()

	var err error
	defer func() {
		if err != nil {
			c.mu.Lock()
			if sb.ingress {
				c.ingressSandbox = nil
			}
			c.mu.Unlock()
		}
	}()

	if err = sb.setupResolutionFiles(); err != nil {
		return nil, err
	}

	if sb.config.useDefaultSandBox {
		c.sboxOnce.Do(func() {
			c.defOsSbox, err = osl.NewSandbox(sb.Key(), false, false)
		})

		if err != nil {
			c.sboxOnce = sync.Once{}
			return nil, fmt.Errorf("failed to create default sandbox: %v", err)
		}

		sb.osSbox = c.defOsSbox
	}

	if sb.osSbox == nil && !sb.config.useExternalKey {
		if sb.osSbox, err = osl.NewSandbox(sb.Key(), !sb.config.useDefaultSandBox, false); err != nil {
			return nil, fmt.Errorf("failed to create new osl sandbox: %v", err)
		}
	}

	if sb.osSbox != nil {
		// Apply operating specific knobs on the load balancer sandbox
		err := sb.osSbox.InvokeFunc(func() {
			sb.osSbox.ApplyOSTweaks(sb.oslTypes)
		})

		if err != nil {
			logrus.Errorf("Failed to apply performance tuning sysctls to the sandbox: %v", err)
		}
		// Keep this just so performance is not changed
		sb.osSbox.ApplyOSTweaks(sb.oslTypes)
	}

	c.mu.Lock()
	c.sandboxes[sb.id] = sb
	c.mu.Unlock()
	defer func() {
		if err != nil {
			c.mu.Lock()
			delete(c.sandboxes, sb.id)
			c.mu.Unlock()
		}
	}()

	err = sb.storeUpdate()
	if err != nil {
		return nil, fmt.Errorf("failed to update the store state of sandbox: %v", err)
	}

	return sb, nil
}

// Sandboxes returns the list of Sandbox(s) managed by this controller.
func (c *Controller) Sandboxes() []*Sandbox {
	c.mu.Lock()
	defer c.mu.Unlock()

	list := make([]*Sandbox, 0, len(c.sandboxes))
	for _, s := range c.sandboxes {
		// Hide stub sandboxes from libnetwork users
		if s.isStub {
			continue
		}

		list = append(list, s)
	}

	return list
}

// WalkSandboxes uses the provided function to walk the Sandbox(s) managed by this controller.
func (c *Controller) WalkSandboxes(walker SandboxWalker) {
	for _, sb := range c.Sandboxes() {
		if walker(sb) {
			return
		}
	}
}

// SandboxByID returns the Sandbox which has the passed id.
// If not found, a [types.NotFoundError] is returned.
func (c *Controller) SandboxByID(id string) (*Sandbox, error) {
	if id == "" {
		return nil, ErrInvalidID(id)
	}
	c.mu.Lock()
	s, ok := c.sandboxes[id]
	c.mu.Unlock()
	if !ok {
		return nil, types.NotFoundErrorf("sandbox %s not found", id)
	}
	return s, nil
}

// SandboxDestroy destroys a sandbox given a container ID.
func (c *Controller) SandboxDestroy(id string) error {
	var sb *Sandbox
	c.mu.Lock()
	for _, s := range c.sandboxes {
		if s.containerID == id {
			sb = s
			break
		}
	}
	c.mu.Unlock()

	// It is not an error if sandbox is not available
	if sb == nil {
		return nil
	}

	return sb.Delete()
}

// SandboxContainerWalker returns a Sandbox Walker function which looks for an existing Sandbox with the passed containerID
func SandboxContainerWalker(out **Sandbox, containerID string) SandboxWalker {
	return func(sb *Sandbox) bool {
		if sb.ContainerID() == containerID {
			*out = sb
			return true
		}
		return false
	}
}

// SandboxKeyWalker returns a Sandbox Walker function which looks for an existing Sandbox with the passed key
func SandboxKeyWalker(out **Sandbox, key string) SandboxWalker {
	return func(sb *Sandbox) bool {
		if sb.Key() == key {
			*out = sb
			return true
		}
		return false
	}
}

func (c *Controller) loadDriver(networkType string) error {
	var err error

	if pg := c.GetPluginGetter(); pg != nil {
		_, err = pg.Get(networkType, driverapi.NetworkPluginEndpointType, plugingetter.Lookup)
	} else {
		_, err = plugins.Get(networkType, driverapi.NetworkPluginEndpointType)
	}

	if err != nil {
		if errors.Cause(err) == plugins.ErrNotFound {
			return types.NotFoundErrorf(err.Error())
		}
		return err
	}

	return nil
}

func (c *Controller) loadIPAMDriver(name string) error {
	var err error

	if pg := c.GetPluginGetter(); pg != nil {
		_, err = pg.Get(name, ipamapi.PluginEndpointType, plugingetter.Lookup)
	} else {
		_, err = plugins.Get(name, ipamapi.PluginEndpointType)
	}

	if err != nil {
		if errors.Cause(err) == plugins.ErrNotFound {
			return types.NotFoundErrorf(err.Error())
		}
		return err
	}

	return nil
}

func (c *Controller) getIPAMDriver(name string) (ipamapi.Ipam, *ipamapi.Capability, error) {
	id, cap := c.drvRegistry.IPAM(name)
	if id == nil {
		// Might be a plugin name. Try loading it
		if err := c.loadIPAMDriver(name); err != nil {
			return nil, nil, err
		}

		// Now that we resolved the plugin, try again looking up the registry
		id, cap = c.drvRegistry.IPAM(name)
		if id == nil {
			return nil, nil, types.BadRequestErrorf("invalid ipam driver: %q", name)
		}
	}

	return id, cap, nil
}

// Stop stops the network controller.
func (c *Controller) Stop() {
	c.closeStores()
	c.stopExternalKeyListener()
	osl.GC()
}

// StartDiagnostic starts the network diagnostic server listening on port.
func (c *Controller) StartDiagnostic(port int) {
	c.mu.Lock()
	if !c.DiagnosticServer.IsDiagnosticEnabled() {
		c.DiagnosticServer.EnableDiagnostic("127.0.0.1", port)
	}
	c.mu.Unlock()
}

// StopDiagnostic stops the network diagnostic server.
func (c *Controller) StopDiagnostic() {
	c.mu.Lock()
	if c.DiagnosticServer.IsDiagnosticEnabled() {
		c.DiagnosticServer.DisableDiagnostic()
	}
	c.mu.Unlock()
}

// IsDiagnosticEnabled returns true if the diagnostic server is running.
func (c *Controller) IsDiagnosticEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.DiagnosticServer.IsDiagnosticEnabled()
}

func (c *Controller) iptablesEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cfg == nil {
		return false
	}
	// parse map cfg["bridge"]["generic"]["EnableIPTable"]
	cfgBridge, ok := c.cfg.DriverCfg["bridge"].(map[string]interface{})
	if !ok {
		return false
	}
	cfgGeneric, ok := cfgBridge[netlabel.GenericData].(options.Generic)
	if !ok {
		return false
	}
	enabled, ok := cfgGeneric["EnableIPTables"].(bool)
	if !ok {
		// unless user explicitly stated, assume iptable is enabled
		enabled = true
	}
	return enabled
}
