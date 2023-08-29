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
	"context"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/cluster"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/diagnostic"
	"github.com/docker/docker/libnetwork/discoverapi"
	"github.com/docker/docker/libnetwork/driverapi"
	remotedriver "github.com/docker/docker/libnetwork/drivers/remote"
	"github.com/docker/docker/libnetwork/drvregistry"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/moby/locker"
	"github.com/pkg/errors"
)

// NetworkWalker is a client provided function which will be used to walk the Networks.
// When the function returns true, the walk will stop.
type NetworkWalker func(nw *Network) bool

// Controller manages networks.
type Controller struct {
	id               string
	drvRegistry      drvregistry.Networks
	ipamRegistry     drvregistry.IPAMs
	sandboxes        map[string]*Sandbox
	cfg              *config.Config
	store            *datastore.Store
	extKeyListener   net.Listener
	watchCh          chan *Endpoint
	unWatchCh        chan *Endpoint
	svcRecords       map[string]*svcInfo
	nmap             map[string]*netWatch
	serviceBindings  map[serviceKey]*service
	ingressSandbox   *Sandbox
	agent            *nwAgent
	networkLocker    *locker.Locker
	agentInitDone    chan struct{}
	agentStopDone    chan struct{}
	keys             []*types.EncryptionKey
	DiagnosticServer *diagnostic.Server
	mu               sync.Mutex

	// FIXME(thaJeztah): defOsSbox is always nil on non-Linux: move these fields to Linux-only files.
	defOsSboxOnce sync.Once
	defOsSbox     *osl.Namespace
}

// New creates a new instance of network controller.
func New(cfgOptions ...config.Option) (*Controller, error) {
	c := &Controller{
		id:               stringid.GenerateRandomID(),
		cfg:              config.New(cfgOptions...),
		sandboxes:        map[string]*Sandbox{},
		svcRecords:       make(map[string]*svcInfo),
		serviceBindings:  make(map[serviceKey]*service),
		agentInitDone:    make(chan struct{}),
		networkLocker:    locker.New(),
		DiagnosticServer: diagnostic.New(),
	}
	c.DiagnosticServer.Init()

	if err := c.initStores(); err != nil {
		return nil, err
	}

	c.drvRegistry.Notify = c

	// External plugins don't need config passed through daemon. They can
	// bootstrap themselves.
	if err := remotedriver.Register(&c.drvRegistry, c.cfg.PluginGetter); err != nil {
		return nil, err
	}

	if err := registerNetworkDrivers(&c.drvRegistry, c.makeDriverConfig); err != nil {
		return nil, err
	}

	if err := initIPAMDrivers(&c.ipamRegistry, c.cfg.PluginGetter, c.cfg.DefaultAddressPool); err != nil {
		return nil, err
	}

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

func (c *Controller) getAgent() *nwAgent {
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

	// Merge in the existing config for this driver.
	for k, v := range c.cfg.DriverConfig(ntype) {
		cfg[k] = v
	}

	if c.cfg.Scope.IsValid() {
		// FIXME: every driver instance constructs a new DataStore
		// instance against the same database. Yikes!
		cfg[netlabel.LocalKVClient] = discoverapi.DatastoreConfigData{
			Scope:    scope.Local,
			Provider: c.cfg.Scope.Client.Provider,
			Address:  c.cfg.Scope.Client.Address,
			Config:   c.cfg.Scope.Client.Config,
		}
	}

	return cfg
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
	c.ipamRegistry.WalkIPAMs(func(name string, driver ipamapi.Ipam, _ *ipamapi.Capability) bool {
		if driver.IsBuiltIn() {
			drivers = append(drivers, name)
		}
		return false
	})
	return drivers
}

func (c *Controller) processNodeDiscovery(nodes []net.IP, add bool) {
	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		if d, ok := driver.(discoverapi.Discover); ok {
			c.pushNodeDiscovery(d, capability, nodes, add)
		}
		return false
	})
}

func (c *Controller) pushNodeDiscovery(d discoverapi.Discover, capability driverapi.Capability, nodes []net.IP, add bool) {
	var self net.IP
	// try swarm-mode config
	if agent := c.getAgent(); agent != nil {
		self = net.ParseIP(agent.advertiseAddr)
	}

	if d == nil || capability.ConnectivityScope != scope.Global || nodes == nil {
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
			log.G(context.TODO()).Debugf("discovery notification error: %v", err)
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
	return c.cfg.PluginGetter
}

func (c *Controller) RegisterDriver(networkType string, driver driverapi.Driver, capability driverapi.Capability) error {
	if d, ok := driver.(discoverapi.Discover); ok {
		c.agentDriverNotify(d)
	}
	return nil
}

// XXX  This should be made driver agnostic.  See comment below.
const overlayDSROptionString = "dsr"

// NewNetwork creates a new network of the specified network type. The options
// are network specific and modeled in a generic way.
func (c *Controller) NewNetwork(networkType, name string, id string, options ...NetworkOption) (_ *Network, retErr error) {
	if id != "" {
		c.networkLocker.Lock(id)
		defer c.networkLocker.Unlock(id) //nolint:errcheck

		if _, err := c.NetworkByID(id); err == nil {
			return nil, NetworkNameError(id)
		}
	}

	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidName(name)
	}

	if id == "" {
		id = stringid.GenerateRandomID()
	}

	defaultIpam := defaultIpamForNetworkType(networkType)
	// Construct the network object
	nw := &Network{
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
	if err := nw.validateConfiguration(); err != nil {
		return nil, err
	}

	// These variables must be defined here, as declaration would otherwise
	// be skipped by the "goto addToStore"
	var (
		caps driverapi.Capability
		err  error

		skipCfgEpCount bool
	)

	// Reset network types, force local scope and skip allocation and
	// plumbing for configuration networks. Reset of the config-only
	// network drivers is needed so that this special network is not
	// usable by old engine versions.
	if nw.configOnly {
		nw.scope = scope.Local
		nw.networkType = "null"
		goto addToStore
	}

	_, caps, err = nw.resolveDriver(nw.networkType, true)
	if err != nil {
		return nil, err
	}

	if nw.scope == scope.Local && caps.DataScope == scope.Global {
		return nil, types.ForbiddenErrorf("cannot downgrade network scope for %s networks", networkType)
	}
	if nw.ingress && caps.DataScope != scope.Global {
		return nil, types.ForbiddenErrorf("Ingress network can only be global scope network")
	}

	// At this point the network scope is still unknown if not set by user
	if (caps.DataScope == scope.Global || nw.scope == scope.Swarm) &&
		!c.isDistributedControl() && !nw.dynamic {
		if c.isManager() {
			// For non-distributed controlled environment, globalscoped non-dynamic networks are redirected to Manager
			return nil, ManagerRedirectError(name)
		}
		return nil, types.ForbiddenErrorf("Cannot create a multi-host network from a worker node. Please create the network from a manager node.")
	}

	if nw.scope == scope.Swarm && c.isDistributedControl() {
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
		configNetwork, err := c.getConfigNetwork(nw.configFrom)
		if err != nil {
			return nil, types.NotFoundErrorf("configuration network %q does not exist", nw.configFrom)
		}
		if err := configNetwork.applyConfigurationTo(nw); err != nil {
			return nil, types.InternalErrorf("Failed to apply configuration: %v", err)
		}
		nw.generic[netlabel.Internal] = nw.internal
		defer func() {
			if retErr == nil && !skipCfgEpCount {
				if err := configNetwork.getEpCnt().IncEndpointCnt(); err != nil {
					log.G(context.TODO()).Warnf("Failed to update reference count for configuration network %q on creation of network %q: %v", configNetwork.Name(), nw.name, err)
				}
			}
		}()
	}

	if err := nw.ipamAllocate(); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			nw.ipamRelease()
		}
	}()

	// Note from thaJeztah to future code visitors, or "future self".
	//
	// This code was previously assigning the error to the global "err"
	// variable (before it was renamed to "retErr"), but in case of a
	// "MaskableError" did not *return* the error:
	// https://github.com/moby/moby/blob/b325dcbff60a04cedbe40eb627465fc7379d05bf/libnetwork/controller.go#L566-L573
	//
	// Depending on code paths further down, that meant that this error
	// was either overwritten by other errors (and thus not handled in
	// defer statements) or handled (if no other code was overwriting it.
	//
	// I suspect this was a bug (but possible without effect), but it could
	// have been intentional. This logic is confusing at least, and even
	// more so combined with the handling in defer statements that check for
	// both the "err" return AND "skipCfgEpCount":
	// https://github.com/moby/moby/blob/b325dcbff60a04cedbe40eb627465fc7379d05bf/libnetwork/controller.go#L586-L602
	//
	// To save future visitors some time to dig up history:
	//
	// - config-only networks were added in 25082206df465d1c11dd1276a65b4a1dc701bd43
	// - the special error-handling and "skipCfgEpcoung" was added in ddd22a819867faa0cd7d12b0c3fad1099ac3eb26
	// - and updated in 87b082f3659f9ec245ab15d781e6bfffced0af83 to don't use string-matching
	//
	// To cut a long story short: if this broke anything, you know who to blame :)
	if err := c.addNetwork(nw); err != nil {
		if _, ok := err.(types.MaskableError); ok { //nolint:gosimple
			// This error can be ignored and set this boolean
			// value to skip a refcount increment for configOnly networks
			skipCfgEpCount = true
		} else {
			return nil, err
		}
	}
	defer func() {
		if retErr != nil {
			if err := nw.deleteNetwork(); err != nil {
				log.G(context.TODO()).Warnf("couldn't roll back driver network on network %s creation failure: %v", nw.name, retErr)
			}
		}
	}()

	// XXX If the driver type is "overlay" check the options for DSR
	// being set.  If so, set the network's load balancing mode to DSR.
	// This should really be done in a network option, but due to
	// time pressure to get this in without adding changes to moby,
	// swarm and CLI, it is being implemented as a driver-specific
	// option.  Unfortunately, drivers can't influence the core
	// "libnetwork.Network" data type.  Hence we need this hack code
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
	if err := c.updateToStore(epCnt); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			if err := c.deleteFromStore(epCnt); err != nil {
				log.G(context.TODO()).Warnf("could not rollback from store, epCnt %v on failure (%v): %v", epCnt, retErr, err)
			}
		}
	}()

	nw.epCnt = epCnt
	if err := c.updateToStore(nw); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			if err := c.deleteFromStore(nw); err != nil {
				log.G(context.TODO()).Warnf("could not rollback from store, network %v on failure (%v): %v", nw, retErr, err)
			}
		}
	}()

	if nw.configOnly {
		return nw, nil
	}

	joinCluster(nw)
	defer func() {
		if retErr != nil {
			nw.cancelDriverWatches()
			if err := nw.leaveCluster(); err != nil {
				log.G(context.TODO()).Warnf("Failed to leave agent cluster on network %s on failure (%v): %v", nw.name, retErr, err)
			}
		}
	}()

	if nw.hasLoadBalancerEndpoint() {
		if err := nw.createLoadBalancerSandbox(); err != nil {
			return nil, err
		}
	}

	if !c.isDistributedControl() {
		c.mu.Lock()
		arrangeIngressFilterRule()
		c.mu.Unlock()
	}

	// Sets up the DOCKER-USER chain for each iptables version (IPv4, IPv6)
	// that's enabled in the controller's configuration.
	for _, ipVersion := range c.enabledIptablesVersions() {
		if err := setupUserChain(ipVersion); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Controller.NewNetwork %s:", name)
		}
	}

	return nw, nil
}

var joinCluster NetworkWalker = func(nw *Network) bool {
	if nw.configOnly {
		return false
	}
	if err := nw.joinCluster(); err != nil {
		log.G(context.TODO()).Errorf("Failed to join network %s (%s) into agent cluster: %v", nw.Name(), nw.ID(), err)
	}
	nw.addDriverWatches()
	return false
}

func (c *Controller) reservePools() {
	networks, err := c.getNetworks()
	if err != nil {
		log.G(context.TODO()).Warnf("Could not retrieve networks from local store during ipam allocation for existing networks: %v", err)
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
			log.G(context.TODO()).Warnf("Failed to allocate ipam pool(s) for network %q (%s): %v", n.Name(), n.ID(), err)
		}
		// Reserve existing endpoints' addresses
		ipam, _, err := n.getController().getIPAMDriver(n.ipamType)
		if err != nil {
			log.G(context.TODO()).Warnf("Failed to retrieve ipam driver for network %q (%s) during address reservation", n.Name(), n.ID())
			continue
		}
		epl, err := n.getEndpointsFromStore()
		if err != nil {
			log.G(context.TODO()).Warnf("Failed to retrieve list of current endpoints on network %q (%s)", n.Name(), n.ID())
			continue
		}
		for _, ep := range epl {
			if ep.Iface() == nil {
				log.G(context.TODO()).Warnf("endpoint interface is empty for %q (%s)", ep.Name(), ep.ID())
				continue
			}
			if err := ep.assignAddress(ipam, true, ep.Iface().AddressIPv6() != nil); err != nil {
				log.G(context.TODO()).Warnf("Failed to reserve current address for endpoint %q (%s) on network %q (%s)",
					ep.Name(), ep.ID(), n.Name(), n.ID())
			}
		}
	}
}

func doReplayPoolReserve(n *Network) bool {
	_, caps, err := n.getController().getIPAMDriver(n.ipamType)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to retrieve ipam driver for network %q (%s): %v", n.Name(), n.ID(), err)
		return false
	}
	return caps.RequiresRequestReplay
}

func (c *Controller) addNetwork(n *Network) error {
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
func (c *Controller) Networks() []*Network {
	var list []*Network

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
func (c *Controller) NetworkByName(name string) (*Network, error) {
	if name == "" {
		return nil, ErrInvalidName(name)
	}
	var n *Network

	c.WalkNetworks(func(current *Network) bool {
		if current.Name() == name {
			n = current
			return true
		}
		return false
	})

	if n == nil {
		return nil, ErrNoSuchNetwork(name)
	}

	return n, nil
}

// NetworkByID returns the Network which has the passed id.
// If not found, the error [ErrNoSuchNetwork] is returned.
func (c *Controller) NetworkByID(id string) (*Network, error) {
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
func (c *Controller) NewSandbox(containerID string, options ...SandboxOption) (_ *Sandbox, retErr error) {
	if containerID == "" {
		return nil, types.InvalidParameterErrorf("invalid container ID")
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

	// Create sandbox and process options first. Key generation depends on an option
	if sb == nil {
		// TODO(thaJeztah): given that a "containerID" must be unique in the list of sandboxes, is there any reason we're not using containerID as sandbox ID on non-Windows?
		sandboxID := containerID
		if runtime.GOOS != "windows" {
			sandboxID = stringid.GenerateRandomID()
		}
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

	defer func() {
		if retErr != nil {
			c.mu.Lock()
			if sb.ingress {
				c.ingressSandbox = nil
			}
			c.mu.Unlock()
		}
	}()

	if err := sb.setupResolutionFiles(); err != nil {
		return nil, err
	}
	if err := c.setupOSLSandbox(sb); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.sandboxes[sb.id] = sb
	c.mu.Unlock()
	defer func() {
		if retErr != nil {
			c.mu.Lock()
			delete(c.sandboxes, sb.id)
			c.mu.Unlock()
		}
	}()

	if err := sb.storeUpdate(); err != nil {
		return nil, fmt.Errorf("failed to update the store state of sandbox: %v", err)
	}

	return sb, nil
}

// GetSandbox returns the Sandbox which has the passed id.
//
// It returns an [ErrInvalidID] when passing an invalid ID, or an
// [types.NotFoundError] if no Sandbox was found for the container.
func (c *Controller) GetSandbox(containerID string) (*Sandbox, error) {
	if containerID == "" {
		return nil, ErrInvalidID("id is empty")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if runtime.GOOS == "windows" {
		// fast-path for Windows, which uses the container ID as sandbox ID.
		if sb := c.sandboxes[containerID]; sb != nil && !sb.isStub {
			return sb, nil
		}
	} else {
		for _, sb := range c.sandboxes {
			if sb.containerID == containerID && !sb.isStub {
				return sb, nil
			}
		}
	}

	return nil, types.NotFoundErrorf("network sandbox for container %s not found", containerID)
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
	id, caps := c.ipamRegistry.IPAM(name)
	if id == nil {
		// Might be a plugin name. Try loading it
		if err := c.loadIPAMDriver(name); err != nil {
			return nil, nil, err
		}

		// Now that we resolved the plugin, try again looking up the registry
		id, caps = c.ipamRegistry.IPAM(name)
		if id == nil {
			return nil, nil, types.InvalidParameterErrorf("invalid ipam driver: %q", name)
		}
	}

	return id, caps, nil
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
