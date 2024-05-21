package bridge

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/libnetwork/portmapper"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

const (
	NetworkType                = "bridge"
	vethPrefix                 = "veth"
	vethLen                    = len(vethPrefix) + 7
	defaultContainerVethPrefix = "eth"
	maxAllocatePortAttempts    = 10
)

const (
	// DefaultGatewayV4AuxKey represents the default-gateway configured by the user
	DefaultGatewayV4AuxKey = "DefaultGatewayIPv4"
	// DefaultGatewayV6AuxKey represents the ipv6 default-gateway configured by the user
	DefaultGatewayV6AuxKey = "DefaultGatewayIPv6"
)

type (
	iptableCleanFunc   func() error
	iptablesCleanFuncs []iptableCleanFunc
)

// configuration info for the "bridge" driver.
type configuration struct {
	EnableIPForwarding  bool
	EnableIPTables      bool
	EnableIP6Tables     bool
	EnableUserlandProxy bool
	UserlandProxyPath   string
}

// networkConfiguration for network specific configuration
type networkConfiguration struct {
	ID                   string
	BridgeName           string
	EnableIPv6           bool
	EnableIPMasquerade   bool
	EnableICC            bool
	InhibitIPv4          bool
	Mtu                  int
	DefaultBindingIP     net.IP
	DefaultBridge        bool
	HostIPv4             net.IP
	HostIPv6             net.IP
	ContainerIfacePrefix string
	// Internal fields set after ipam data parsing
	AddressIPv4        *net.IPNet
	AddressIPv6        *net.IPNet
	DefaultGatewayIPv4 net.IP
	DefaultGatewayIPv6 net.IP
	dbIndex            uint64
	dbExists           bool
	Internal           bool

	BridgeIfaceCreator ifaceCreator
}

// ifaceCreator represents how the bridge interface was created
type ifaceCreator int8

const (
	ifaceCreatorUnknown ifaceCreator = iota
	ifaceCreatedByLibnetwork
	ifaceCreatedByUser
)

// containerConfiguration represents the user specified configuration for a container
type containerConfiguration struct {
	ParentEndpoints []string
	ChildEndpoints  []string
}

// connectivityConfiguration represents the user specified configuration regarding the external connectivity
type connectivityConfiguration struct {
	PortBindings []types.PortBinding
	ExposedPorts []types.TransportPort
}

type bridgeEndpoint struct {
	id              string
	nid             string
	srcName         string
	addr            *net.IPNet
	addrv6          *net.IPNet
	macAddress      net.HardwareAddr
	containerConfig *containerConfiguration
	extConnConfig   *connectivityConfiguration
	portMapping     []types.PortBinding // Operation port bindings
	dbIndex         uint64
	dbExists        bool
}

type bridgeNetwork struct {
	id            string
	bridge        *bridgeInterface // The bridge's L3 interface
	config        *networkConfiguration
	endpoints     map[string]*bridgeEndpoint // key: endpoint id
	portMapper    *portmapper.PortMapper
	portMapperV6  *portmapper.PortMapper
	driver        *driver // The network's driver
	iptCleanFuncs iptablesCleanFuncs
	sync.Mutex
}

type driver struct {
	config            configuration
	natChain          *iptables.ChainInfo
	filterChain       *iptables.ChainInfo
	isolationChain1   *iptables.ChainInfo
	isolationChain2   *iptables.ChainInfo
	natChainV6        *iptables.ChainInfo
	filterChainV6     *iptables.ChainInfo
	isolationChain1V6 *iptables.ChainInfo
	isolationChain2V6 *iptables.ChainInfo
	networks          map[string]*bridgeNetwork
	store             *datastore.Store
	nlh               *netlink.Handle
	configNetwork     sync.Mutex
	portAllocator     *portallocator.PortAllocator // Overridable for tests.
	sync.Mutex
}

// New constructs a new bridge driver
func newDriver() *driver {
	return &driver{
		networks:      map[string]*bridgeNetwork{},
		portAllocator: portallocator.Get(),
	}
}

// Register registers a new instance of bridge driver.
func Register(r driverapi.Registerer, config map[string]interface{}) error {
	d := newDriver()
	if err := d.configure(config); err != nil {
		return err
	}
	return r.RegisterDriver(NetworkType, d, driverapi.Capability{
		DataScope:         scope.Local,
		ConnectivityScope: scope.Local,
	})
}

// The behaviour of previous implementations of bridge subnet prefix assignment
// is preserved here...
//
// The LL prefix, 'fe80::/64' can be used as an IPAM pool. Linux always assigns
// link-local addresses with this prefix. But, pool-assigned addresses are very
// unlikely to conflict.
//
// Don't allow a nonstandard LL subnet to overlap with 'fe80::/64'. For example,
// if the config asked for subnet prefix 'fe80::/80', the bridge and its
// containers would each end up with two LL addresses, Linux's '/64' and one from
// the IPAM pool claiming '/80'. Although the specified prefix length must not
// affect the host's determination of whether the address is on-link and to be
// added to the interface's Prefix List (RFC-5942), differing prefix lengths
// would be confusing and have been disallowed by earlier implementations of
// bridge address assignment.
func validateIPv6Subnet(addr netip.Prefix) error {
	if !addr.Addr().Is6() || addr.Addr().Is4In6() {
		return fmt.Errorf("'%s' is not a valid IPv6 subnet", addr)
	}
	if addr.Addr().IsMulticast() {
		return fmt.Errorf("multicast subnet '%s' is not allowed", addr)
	}
	if addr.Masked() != linkLocalPrefix && linkLocalPrefix.Overlaps(addr) {
		return fmt.Errorf("'%s' clashes with the Link-Local prefix 'fe80::/64'", addr)
	}
	return nil
}

// ValidateFixedCIDRV6 checks that val is an IPv6 address and prefix length that
// does not overlap with the link local subnet prefix 'fe80::/64'.
func ValidateFixedCIDRV6(val string) error {
	if val == "" {
		return nil
	}
	prefix, err := netip.ParsePrefix(val)
	if err == nil {
		err = validateIPv6Subnet(prefix)
	}
	return errdefs.InvalidParameter(errors.Wrap(err, "invalid fixed-cidr-v6"))
}

// Validate performs a static validation on the network configuration parameters.
// Whatever can be assessed a priori before attempting any programming.
func (c *networkConfiguration) Validate() error {
	if c.Mtu < 0 {
		return ErrInvalidMtu(c.Mtu)
	}

	// If bridge v4 subnet is specified
	if c.AddressIPv4 != nil {
		// If default gw is specified, it must be part of bridge subnet
		if c.DefaultGatewayIPv4 != nil {
			if !c.AddressIPv4.Contains(c.DefaultGatewayIPv4) {
				return &ErrInvalidGateway{}
			}
		}
	}

	if c.EnableIPv6 {
		// If IPv6 is enabled, AddressIPv6 must have been configured.
		if c.AddressIPv6 == nil {
			return errdefs.System(errors.New("no IPv6 address was allocated for the bridge"))
		}
		// AddressIPv6 must be IPv6, and not overlap with the LL subnet prefix.
		addr, ok := netiputil.ToPrefix(c.AddressIPv6)
		if !ok {
			return errdefs.InvalidParameter(fmt.Errorf("invalid IPv6 address '%s'", c.AddressIPv6))
		}
		if err := validateIPv6Subnet(addr); err != nil {
			return errdefs.InvalidParameter(err)
		}
		// If a default gw is specified, it must belong to AddressIPv6's subnet
		if c.DefaultGatewayIPv6 != nil && !c.AddressIPv6.Contains(c.DefaultGatewayIPv6) {
			return &ErrInvalidGateway{}
		}
	}

	return nil
}

// Conflicts check if two NetworkConfiguration objects overlap
func (c *networkConfiguration) Conflicts(o *networkConfiguration) error {
	if o == nil {
		return errors.New("same configuration")
	}

	// Also empty, because only one network with empty name is allowed
	if c.BridgeName == o.BridgeName {
		return errors.New("networks have same bridge name")
	}

	// They must be in different subnets
	if (c.AddressIPv4 != nil && o.AddressIPv4 != nil) &&
		(c.AddressIPv4.Contains(o.AddressIPv4.IP) || o.AddressIPv4.Contains(c.AddressIPv4.IP)) {
		return errors.New("networks have overlapping IPv4")
	}

	// They must be in different v6 subnets
	if (c.AddressIPv6 != nil && o.AddressIPv6 != nil) &&
		(c.AddressIPv6.Contains(o.AddressIPv6.IP) || o.AddressIPv6.Contains(c.AddressIPv6.IP)) {
		return errors.New("networks have overlapping IPv6")
	}

	return nil
}

func (c *networkConfiguration) fromLabels(labels map[string]string) error {
	var err error
	for label, value := range labels {
		switch label {
		case BridgeName:
			c.BridgeName = value
		case netlabel.DriverMTU:
			if c.Mtu, err = strconv.Atoi(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case netlabel.EnableIPv6:
			if c.EnableIPv6, err = strconv.ParseBool(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case EnableIPMasquerade:
			if c.EnableIPMasquerade, err = strconv.ParseBool(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case EnableICC:
			if c.EnableICC, err = strconv.ParseBool(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case InhibitIPv4:
			if c.InhibitIPv4, err = strconv.ParseBool(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case DefaultBridge:
			if c.DefaultBridge, err = strconv.ParseBool(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case DefaultBindingIP:
			if c.DefaultBindingIP = net.ParseIP(value); c.DefaultBindingIP == nil {
				return parseErr(label, value, "nil ip")
			}
		case netlabel.ContainerIfacePrefix:
			c.ContainerIfacePrefix = value
		case netlabel.HostIPv4:
			if c.HostIPv4 = net.ParseIP(value); c.HostIPv4 == nil {
				return parseErr(label, value, "nil ip")
			}
		case netlabel.HostIPv6:
			if c.HostIPv6 = net.ParseIP(value); c.HostIPv6 == nil {
				return parseErr(label, value, "nil ip")
			}
		}
	}

	return nil
}

func parseErr(label, value, errString string) error {
	return types.InvalidParameterErrorf("failed to parse %s value: %v (%s)", label, value, errString)
}

func (n *bridgeNetwork) registerIptCleanFunc(clean iptableCleanFunc) {
	n.iptCleanFuncs = append(n.iptCleanFuncs, clean)
}

func (n *bridgeNetwork) getDriverChains(version iptables.IPVersion) (*iptables.ChainInfo, *iptables.ChainInfo, *iptables.ChainInfo, *iptables.ChainInfo, error) {
	n.Lock()
	defer n.Unlock()

	if n.driver == nil {
		return nil, nil, nil, nil, types.InvalidParameterErrorf("no driver found")
	}

	if version == iptables.IPv6 {
		return n.driver.natChainV6, n.driver.filterChainV6, n.driver.isolationChain1V6, n.driver.isolationChain2V6, nil
	}

	return n.driver.natChain, n.driver.filterChain, n.driver.isolationChain1, n.driver.isolationChain2, nil
}

func (n *bridgeNetwork) getNetworkBridgeName() string {
	n.Lock()
	config := n.config
	n.Unlock()

	return config.BridgeName
}

func (n *bridgeNetwork) getEndpoint(eid string) (*bridgeEndpoint, error) {
	if eid == "" {
		return nil, InvalidEndpointIDError(eid)
	}

	n.Lock()
	defer n.Unlock()
	if ep, ok := n.endpoints[eid]; ok {
		return ep, nil
	}

	return nil, nil
}

// Install/Removes the iptables rules needed to isolate this network
// from each of the other networks
func (n *bridgeNetwork) isolateNetwork(enable bool) error {
	n.Lock()
	thisConfig := n.config
	n.Unlock()

	if thisConfig.Internal {
		return nil
	}

	// Install the rules to isolate this network against each of the other networks
	if n.driver.config.EnableIP6Tables {
		err := setINC(iptables.IPv6, thisConfig.BridgeName, enable)
		if err != nil {
			return err
		}
	}

	if n.driver.config.EnableIPTables {
		return setINC(iptables.IPv4, thisConfig.BridgeName, enable)
	}
	return nil
}

func (d *driver) configure(option map[string]interface{}) error {
	var (
		config            configuration
		err               error
		natChain          *iptables.ChainInfo
		filterChain       *iptables.ChainInfo
		isolationChain1   *iptables.ChainInfo
		isolationChain2   *iptables.ChainInfo
		natChainV6        *iptables.ChainInfo
		filterChainV6     *iptables.ChainInfo
		isolationChain1V6 *iptables.ChainInfo
		isolationChain2V6 *iptables.ChainInfo
	)

	switch opt := option[netlabel.GenericData].(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &configuration{})
		if err != nil {
			return err
		}
		config = *opaqueConfig.(*configuration)
	case *configuration:
		config = *opt
	case nil:
		// No GenericData option set. Use defaults.
	default:
		return &ErrInvalidDriverConfig{}
	}

	if config.EnableIPTables || config.EnableIP6Tables {
		if _, err := os.Stat("/proc/sys/net/bridge"); err != nil {
			if out, err := exec.Command("modprobe", "-va", "bridge", "br_netfilter").CombinedOutput(); err != nil {
				log.G(context.TODO()).Warnf("Running modprobe bridge br_netfilter failed with message: %s, error: %v", out, err)
			}
		}
	}

	if config.EnableIPTables {
		removeIPChains(iptables.IPv4)

		natChain, filterChain, isolationChain1, isolationChain2, err = setupIPChains(config, iptables.IPv4)
		if err != nil {
			return err
		}

		// Make sure on firewall reload, first thing being re-played is chains creation
		iptables.OnReloaded(func() {
			log.G(context.TODO()).Debugf("Recreating iptables chains on firewall reload")
			if _, _, _, _, err := setupIPChains(config, iptables.IPv4); err != nil {
				log.G(context.TODO()).WithError(err).Error("Error reloading iptables chains")
			}
		})
	}

	if config.EnableIP6Tables {
		removeIPChains(iptables.IPv6)

		natChainV6, filterChainV6, isolationChain1V6, isolationChain2V6, err = setupIPChains(config, iptables.IPv6)
		if err != nil {
			return err
		}

		// Make sure on firewall reload, first thing being re-played is chains creation
		iptables.OnReloaded(func() {
			log.G(context.TODO()).Debugf("Recreating ip6tables chains on firewall reload")
			if _, _, _, _, err := setupIPChains(config, iptables.IPv6); err != nil {
				log.G(context.TODO()).WithError(err).Error("Error reloading ip6tables chains")
			}
		})
	}

	if config.EnableIPForwarding {
		err = setupIPForwarding(config.EnableIPTables, config.EnableIP6Tables)
		if err != nil {
			log.G(context.TODO()).Warn(err)
			return err
		}
	}

	d.Lock()
	d.natChain = natChain
	d.filterChain = filterChain
	d.isolationChain1 = isolationChain1
	d.isolationChain2 = isolationChain2
	d.natChainV6 = natChainV6
	d.filterChainV6 = filterChainV6
	d.isolationChain1V6 = isolationChain1V6
	d.isolationChain2V6 = isolationChain2V6
	d.config = config
	d.Unlock()

	return d.initStore(option)
}

func (d *driver) getNetwork(id string) (*bridgeNetwork, error) {
	d.Lock()
	defer d.Unlock()

	if id == "" {
		return nil, types.InvalidParameterErrorf("invalid network id: %s", id)
	}

	if nw, ok := d.networks[id]; ok {
		return nw, nil
	}

	return nil, types.NotFoundErrorf("network not found: %s", id)
}

func parseNetworkGenericOptions(data interface{}) (*networkConfiguration, error) {
	var (
		err    error
		config *networkConfiguration
	)

	switch opt := data.(type) {
	case *networkConfiguration:
		config = opt
	case map[string]string:
		config = &networkConfiguration{
			EnableICC:          true,
			EnableIPMasquerade: true,
		}
		err = config.fromLabels(opt)
	case options.Generic:
		var opaqueConfig interface{}
		if opaqueConfig, err = options.GenerateFromModel(opt, config); err == nil {
			config = opaqueConfig.(*networkConfiguration)
		}
	default:
		err = types.InvalidParameterErrorf("do not recognize network configuration format: %T", opt)
	}

	return config, err
}

func (c *networkConfiguration) processIPAM(id string, ipamV4Data, ipamV6Data []driverapi.IPAMData) error {
	if len(ipamV4Data) > 1 || len(ipamV6Data) > 1 {
		return types.ForbiddenErrorf("bridge driver doesn't support multiple subnets")
	}

	if len(ipamV4Data) == 0 {
		return types.InvalidParameterErrorf("bridge network %s requires ipv4 configuration", id)
	}

	if ipamV4Data[0].Gateway != nil {
		c.AddressIPv4 = types.GetIPNetCopy(ipamV4Data[0].Gateway)
	}

	if gw, ok := ipamV4Data[0].AuxAddresses[DefaultGatewayV4AuxKey]; ok {
		c.DefaultGatewayIPv4 = gw.IP
	}

	if len(ipamV6Data) > 0 {
		c.AddressIPv6 = ipamV6Data[0].Pool

		if ipamV6Data[0].Gateway != nil {
			c.AddressIPv6 = types.GetIPNetCopy(ipamV6Data[0].Gateway)
		}

		if gw, ok := ipamV6Data[0].AuxAddresses[DefaultGatewayV6AuxKey]; ok {
			c.DefaultGatewayIPv6 = gw.IP
		}
	}

	return nil
}

func parseNetworkOptions(id string, option options.Generic) (*networkConfiguration, error) {
	var (
		err    error
		config = &networkConfiguration{}
	)

	// Parse generic label first, config will be re-assigned
	if genData, ok := option[netlabel.GenericData]; ok && genData != nil {
		if config, err = parseNetworkGenericOptions(genData); err != nil {
			return nil, err
		}
	}

	// Process well-known labels next
	if val, ok := option[netlabel.EnableIPv6]; ok {
		config.EnableIPv6 = val.(bool)
	}

	if val, ok := option[netlabel.Internal]; ok {
		if internal, ok := val.(bool); ok && internal {
			config.Internal = true
		}
	}

	if config.BridgeName == "" && !config.DefaultBridge {
		config.BridgeName = "br-" + id[:12]
	}

	exists, err := bridgeInterfaceExists(config.BridgeName)
	if err != nil {
		return nil, err
	}

	if !exists {
		config.BridgeIfaceCreator = ifaceCreatedByLibnetwork
	} else {
		config.BridgeIfaceCreator = ifaceCreatedByUser
	}

	config.ID = id
	return config, nil
}

// Return a slice of networks over which caller can iterate safely
func (d *driver) getNetworks() []*bridgeNetwork {
	d.Lock()
	defer d.Unlock()

	ls := make([]*bridgeNetwork, 0, len(d.networks))
	for _, nw := range d.networks {
		ls = append(ls, nw)
	}
	return ls
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
}

func (d *driver) DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string) {
	return "", nil
}

// Create a new network using bridge plugin
func (d *driver) CreateNetwork(id string, option map[string]interface{}, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	if len(ipV4Data) == 0 || ipV4Data[0].Pool.String() == "0.0.0.0/0" {
		return types.InvalidParameterErrorf("ipv4 pool is empty")
	}
	// Sanity checks
	d.Lock()
	if _, ok := d.networks[id]; ok {
		d.Unlock()
		return types.ForbiddenErrorf("network %s exists", id)
	}
	d.Unlock()

	// Parse the config.
	config, err := parseNetworkOptions(id, option)
	if err != nil {
		return err
	}

	// Add IP addresses/gateways to the configuration.
	if err = config.processIPAM(id, ipV4Data, ipV6Data); err != nil {
		return err
	}

	// Validate the configuration
	if err = config.Validate(); err != nil {
		return err
	}

	// start the critical section, from this point onward we are dealing with the list of networks
	// so to be consistent we cannot allow that the list changes
	d.configNetwork.Lock()
	defer d.configNetwork.Unlock()

	// check network conflicts
	if err = d.checkConflict(config); err != nil {
		return err
	}

	// there is no conflict, now create the network
	if err = d.createNetwork(config); err != nil {
		return err
	}

	return d.storeUpdate(config)
}

func (d *driver) checkConflict(config *networkConfiguration) error {
	networkList := d.getNetworks()
	for _, nw := range networkList {
		nw.Lock()
		nwConfig := nw.config
		nw.Unlock()
		if err := nwConfig.Conflicts(config); err != nil {
			return types.ForbiddenErrorf("cannot create network %s (%s): conflicts with network %s (%s): %s",
				config.ID, config.BridgeName, nwConfig.ID, nwConfig.BridgeName, err.Error())
		}
	}
	return nil
}

func (d *driver) createNetwork(config *networkConfiguration) (err error) {
	// Initialize handle when needed
	d.Lock()
	if d.nlh == nil {
		d.nlh = ns.NlHandle()
	}
	d.Unlock()

	// Create or retrieve the bridge L3 interface
	bridgeIface, err := newInterface(d.nlh, config)
	if err != nil {
		return err
	}

	// Create and set network handler in driver
	network := &bridgeNetwork{
		id:           config.ID,
		endpoints:    make(map[string]*bridgeEndpoint),
		config:       config,
		portMapper:   portmapper.NewWithPortAllocator(d.portAllocator, d.config.UserlandProxyPath),
		portMapperV6: portmapper.NewWithPortAllocator(d.portAllocator, d.config.UserlandProxyPath),
		bridge:       bridgeIface,
		driver:       d,
	}

	d.Lock()
	d.networks[config.ID] = network
	d.Unlock()

	// On failure make sure to reset driver network handler to nil
	defer func() {
		if err != nil {
			d.Lock()
			delete(d.networks, config.ID)
			d.Unlock()
		}
	}()

	// Add inter-network communication rules.
	setupNetworkIsolationRules := func(config *networkConfiguration, i *bridgeInterface) error {
		if err := network.isolateNetwork(true); err != nil {
			if err = network.isolateNetwork(false); err != nil {
				log.G(context.TODO()).Warnf("Failed on removing the inter-network iptables rules on cleanup: %v", err)
			}
			return err
		}
		// register the cleanup function
		network.registerIptCleanFunc(func() error {
			return network.isolateNetwork(false)
		})
		return nil
	}

	// Prepare the bridge setup configuration
	bridgeSetup := newBridgeSetup(config, bridgeIface)

	// If the bridge interface doesn't exist, we need to start the setup steps
	// by creating a new device and assigning it an IPv4 address.
	bridgeAlreadyExists := bridgeIface.exists()
	if !bridgeAlreadyExists {
		bridgeSetup.queueStep(setupDevice)
		bridgeSetup.queueStep(setupDefaultSysctl)
	}

	// For the default bridge, set expected sysctls
	if config.DefaultBridge {
		bridgeSetup.queueStep(setupDefaultSysctl)
	}

	// Always set the bridge's MTU if specified. This is purely cosmetic; a bridge's
	// MTU is the min MTU of device connected to it, and MTU will be set on each
	// 'veth'. But, for a non-default MTU, the bridge's MTU will look wrong until a
	// container is attached.
	if config.Mtu > 0 {
		bridgeSetup.queueStep(setupMTU)
	}

	// Even if a bridge exists try to setup IPv4.
	bridgeSetup.queueStep(setupBridgeIPv4)

	enableIPv6Forwarding := config.EnableIPv6 && d.config.EnableIPForwarding

	// Conditionally queue setup steps depending on configuration values.
	for _, step := range []struct {
		Condition bool
		Fn        setupStep
	}{
		// Enable IPv6 on the bridge if required. We do this even for a
		// previously  existing bridge, as it may be here from a previous
		// installation where IPv6 wasn't supported yet and needs to be
		// assigned an IPv6 link-local address.
		{config.EnableIPv6, setupBridgeIPv6},

		// Ensure the bridge has the expected IPv4 addresses in the case of a previously
		// existing device.
		{bridgeAlreadyExists && !config.InhibitIPv4, setupVerifyAndReconcileIPv4},

		// Enable IPv6 Forwarding
		{enableIPv6Forwarding, setupIPv6Forwarding},

		// Setup Loopback Addresses Routing
		{!d.config.EnableUserlandProxy, setupLoopbackAddressesRouting},

		// Setup IPTables.
		{d.config.EnableIPTables, network.setupIP4Tables},

		// Setup IP6Tables.
		{config.EnableIPv6 && d.config.EnableIP6Tables, network.setupIP6Tables},

		// We want to track firewalld configuration so that
		// if it is started/reloaded, the rules can be applied correctly
		{d.config.EnableIPTables, network.setupFirewalld},
		// same for IPv6
		{config.EnableIPv6 && d.config.EnableIP6Tables, network.setupFirewalld6},

		// Setup DefaultGatewayIPv4
		{config.DefaultGatewayIPv4 != nil, setupGatewayIPv4},

		// Setup DefaultGatewayIPv6
		{config.DefaultGatewayIPv6 != nil, setupGatewayIPv6},

		// Add inter-network communication rules.
		{d.config.EnableIPTables, setupNetworkIsolationRules},

		// Configure bridge networking filtering if ICC is off and IP tables are enabled
		{!config.EnableICC && d.config.EnableIPTables, setupBridgeNetFiltering},
	} {
		if step.Condition {
			bridgeSetup.queueStep(step.Fn)
		}
	}

	// Apply the prepared list of steps, and abort at the first error.
	bridgeSetup.queueStep(setupDeviceUp)
	return bridgeSetup.apply()
}

func (d *driver) DeleteNetwork(nid string) error {
	d.configNetwork.Lock()
	defer d.configNetwork.Unlock()

	return d.deleteNetwork(nid)
}

func (d *driver) deleteNetwork(nid string) error {
	var err error

	// Get network handler and remove it from driver
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.InternalMaskableErrorf("network %s does not exist", nid)
	}

	n.Lock()
	config := n.config
	n.Unlock()

	// delele endpoints belong to this network
	for _, ep := range n.endpoints {
		if err := n.releasePorts(ep); err != nil {
			log.G(context.TODO()).Warn(err)
		}
		if link, err := d.nlh.LinkByName(ep.srcName); err == nil {
			if err := d.nlh.LinkDel(link); err != nil {
				log.G(context.TODO()).WithError(err).Errorf("Failed to delete interface (%s)'s link on endpoint (%s) delete", ep.srcName, ep.id)
			}
		}

		if err := d.storeDelete(ep); err != nil {
			log.G(context.TODO()).Warnf("Failed to remove bridge endpoint %.7s from store: %v", ep.id, err)
		}
	}

	d.Lock()
	delete(d.networks, nid)
	d.Unlock()

	// On failure set network handler back in driver, but
	// only if is not already taken over by some other thread
	defer func() {
		if err != nil {
			d.Lock()
			if _, ok := d.networks[nid]; !ok {
				d.networks[nid] = n
			}
			d.Unlock()
		}
	}()

	switch config.BridgeIfaceCreator {
	case ifaceCreatedByLibnetwork, ifaceCreatorUnknown:
		// We only delete the bridge if it was created by the bridge driver and
		// it is not the default one (to keep the backward compatible behavior.)
		if !config.DefaultBridge {
			if err := d.nlh.LinkDel(n.bridge.Link); err != nil {
				log.G(context.TODO()).Warnf("Failed to remove bridge interface %s on network %s delete: %v", config.BridgeName, nid, err)
			}
		}
	case ifaceCreatedByUser:
		// Don't delete the bridge interface if it was not created by libnetwork.
	}

	// clean all relevant iptables rules
	for _, cleanFunc := range n.iptCleanFuncs {
		if errClean := cleanFunc(); errClean != nil {
			log.G(context.TODO()).Warnf("Failed to clean iptables rules for bridge network: %v", errClean)
		}
	}
	return d.storeDelete(config)
}

func addToBridge(nlh *netlink.Handle, ifaceName, bridgeName string) error {
	lnk, err := nlh.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("could not find interface %s: %v", ifaceName, err)
	}
	if err := nlh.LinkSetMaster(lnk, &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: bridgeName}}); err != nil {
		log.G(context.TODO()).WithError(err).Errorf("Failed to add %s to bridge via netlink", ifaceName)
		return err
	}
	return nil
}

func setHairpinMode(nlh *netlink.Handle, link netlink.Link, enable bool) error {
	err := nlh.LinkSetHairpin(link, enable)
	if err != nil {
		return fmt.Errorf("unable to set hairpin mode on %s via netlink: %v",
			link.Attrs().Name, err)
	}
	return nil
}

func (d *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, _ map[string]interface{}) error {
	if ifInfo == nil {
		return errors.New("invalid interface info passed")
	}

	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	dconfig := d.config
	d.Unlock()

	if !ok {
		return types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}

	// Endpoint with that id exists either on desired or other sandbox
	if ep != nil {
		return driverapi.ErrEndpointExists(eid)
	}

	// Create and add the endpoint
	n.Lock()
	endpoint := &bridgeEndpoint{id: eid, nid: nid}
	n.endpoints[eid] = endpoint
	n.Unlock()

	// On failure make sure to remove the endpoint
	defer func() {
		if err != nil {
			n.Lock()
			delete(n.endpoints, eid)
			n.Unlock()
		}
	}()

	// Generate a name for what will be the host side pipe interface
	hostIfName, err := netutils.GenerateIfaceName(d.nlh, vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate a name for what will be the sandbox side pipe interface
	containerIfName, err := netutils.GenerateIfaceName(d.nlh, vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIfName, TxQLen: 0},
		PeerName:  containerIfName,
	}
	if err = d.nlh.LinkAdd(veth); err != nil {
		return types.InternalErrorf("failed to add the host (%s) <=> sandbox (%s) pair interfaces: %v", hostIfName, containerIfName, err)
	}

	// Get the host side pipe interface handler
	host, err := d.nlh.LinkByName(hostIfName)
	if err != nil {
		return types.InternalErrorf("failed to find host side interface %s: %v", hostIfName, err)
	}
	defer func() {
		if err != nil {
			if err := d.nlh.LinkDel(host); err != nil {
				log.G(context.TODO()).WithError(err).Warnf("Failed to delete host side interface (%s)'s link", hostIfName)
			}
		}
	}()

	// Get the sandbox side pipe interface handler
	sbox, err := d.nlh.LinkByName(containerIfName)
	if err != nil {
		return types.InternalErrorf("failed to find sandbox side interface %s: %v", containerIfName, err)
	}
	defer func() {
		if err != nil {
			if err := d.nlh.LinkDel(sbox); err != nil {
				log.G(context.TODO()).WithError(err).Warnf("Failed to delete sandbox side interface (%s)'s link", containerIfName)
			}
		}
	}()

	n.Lock()
	config := n.config
	n.Unlock()

	// Add bridge inherited attributes to pipe interfaces
	if config.Mtu != 0 {
		err = d.nlh.LinkSetMTU(host, config.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on host interface %s: %v", hostIfName, err)
		}
		err = d.nlh.LinkSetMTU(sbox, config.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on sandbox interface %s: %v", containerIfName, err)
		}
	}

	// Attach host side pipe interface into the bridge
	if err = addToBridge(d.nlh, hostIfName, config.BridgeName); err != nil {
		return fmt.Errorf("adding interface %s to bridge %s failed: %v", hostIfName, config.BridgeName, err)
	}

	if !dconfig.EnableUserlandProxy {
		err = setHairpinMode(d.nlh, host, true)
		if err != nil {
			return err
		}
	}

	// Store the sandbox side pipe interface parameters
	endpoint.srcName = containerIfName
	endpoint.macAddress = ifInfo.MacAddress()
	endpoint.addr = ifInfo.Address()
	endpoint.addrv6 = ifInfo.AddressIPv6()

	// We assign a default MAC address derived from the IP address to make sure
	// that if a container is disconnected and reconnected in a short timeframe,
	// stale ARP entries will still point to the right container.
	if endpoint.macAddress == nil {
		endpoint.macAddress = netutils.GenerateMACFromIP(endpoint.addr.IP)
		if err := ifInfo.SetMacAddress(endpoint.macAddress); err != nil {
			return err
		}
	}

	// Up the host interface after finishing all netlink configuration
	if err = d.nlh.LinkSetUp(host); err != nil {
		return fmt.Errorf("could not set link up for host interface %s: %v", hostIfName, err)
	}

	if endpoint.addrv6 == nil && config.EnableIPv6 {
		var ip6 net.IP
		network := n.bridge.bridgeIPv6
		if config.AddressIPv6 != nil {
			network = config.AddressIPv6
		}
		ones, _ := network.Mask.Size()
		if ones > 80 {
			err = types.ForbiddenErrorf("Cannot self generate an IPv6 address on network %v: At least 48 host bits are needed.", network)
			return err
		}

		ip6 = make(net.IP, len(network.IP))
		copy(ip6, network.IP)
		for i, h := range endpoint.macAddress {
			ip6[i+10] = h
		}

		endpoint.addrv6 = &net.IPNet{IP: ip6, Mask: network.Mask}
		if err = ifInfo.SetIPAddress(endpoint.addrv6); err != nil {
			return err
		}
	}

	if err = d.storeUpdate(endpoint); err != nil {
		return fmt.Errorf("failed to save bridge endpoint %.7s to store: %v", endpoint.id, err)
	}

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	var err error

	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.InternalMaskableErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity Check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check endpoint id and if an endpoint is actually there
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}
	if ep == nil {
		return EndpointNotFoundError(eid)
	}

	// Remove it
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()

	// On failure make sure to set back ep in n.endpoints, but only
	// if it hasn't been taken over already by some other thread.
	defer func() {
		if err != nil {
			n.Lock()
			if _, ok := n.endpoints[eid]; !ok {
				n.endpoints[eid] = ep
			}
			n.Unlock()
		}
	}()

	// Try removal of link. Discard error: it is a best effort.
	// Also make sure defer does not see this error either.
	if link, err := d.nlh.LinkByName(ep.srcName); err == nil {
		if err := d.nlh.LinkDel(link); err != nil {
			log.G(context.TODO()).WithError(err).Errorf("Failed to delete interface (%s)'s link on endpoint (%s) delete", ep.srcName, ep.id)
		}
	}

	if err := d.storeDelete(ep); err != nil {
		log.G(context.TODO()).Warnf("Failed to remove bridge endpoint %.7s from store: %v", ep.id, err)
	}

	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()
	if !ok {
		return nil, types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return nil, driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return nil, InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, driverapi.ErrNoEndpoint(eid)
	}

	m := make(map[string]interface{})

	if ep.extConnConfig != nil && ep.extConnConfig.ExposedPorts != nil {
		// Return a copy of the config data
		epc := make([]types.TransportPort, 0, len(ep.extConnConfig.ExposedPorts))
		for _, tp := range ep.extConnConfig.ExposedPorts {
			epc = append(epc, tp.GetCopy())
		}
		m[netlabel.ExposedPorts] = epc
	}

	if ep.portMapping != nil {
		// Return a copy of the operational data
		pmc := make([]types.PortBinding, 0, len(ep.portMapping))
		for _, pm := range ep.portMapping {
			pmc = append(pmc, pm.GetCopy())
		}
		m[netlabel.PortMap] = pmc
	}

	if len(ep.macAddress) != 0 {
		m[netlabel.MacAddress] = ep.macAddress
	}

	return m, nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	endpoint.containerConfig, err = parseContainerOptions(options)
	if err != nil {
		return err
	}

	iNames := jinfo.InterfaceName()
	containerVethPrefix := defaultContainerVethPrefix
	if network.config.ContainerIfacePrefix != "" {
		containerVethPrefix = network.config.ContainerIfacePrefix
	}
	if err := iNames.SetNames(endpoint.srcName, containerVethPrefix); err != nil {
		return err
	}

	if !network.config.Internal {
		if err := jinfo.SetGateway(network.bridge.gatewayIPv4); err != nil {
			return err
		}
		if err := jinfo.SetGatewayIPv6(network.bridge.gatewayIPv6); err != nil {
			return err
		}
	}

	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return types.InternalMaskableErrorf("%s", err)
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	if !network.config.EnableICC {
		if err = d.link(network, endpoint, false); err != nil {
			return err
		}
	}

	return nil
}

func (d *driver) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	endpoint.extConnConfig, err = parseConnectivityOptions(options)
	if err != nil {
		return err
	}

	// Program any required port mapping and store them in the endpoint
	endpoint.portMapping, err = network.allocatePorts(endpoint, network.config.DefaultBindingIP, d.config.EnableUserlandProxy)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if e := network.releasePorts(endpoint); e != nil {
				log.G(context.TODO()).Errorf("Failed to release ports allocated for the bridge endpoint %s on failure %v because of %v",
					eid, err, e)
			}
			endpoint.portMapping = nil
		}
	}()

	// Clean the connection tracker state of the host for the specific endpoint. This is needed because some flows may
	// be bound to the local proxy, or to the host (for UDP packets), and won't be redirected to the new endpoints.
	clearConntrackEntries(d.nlh, endpoint)

	if err = d.storeUpdate(endpoint); err != nil {
		return fmt.Errorf("failed to update bridge endpoint %.7s to store: %v", endpoint.id, err)
	}

	if !network.config.EnableICC {
		return d.link(network, endpoint, true)
	}

	return nil
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	err = network.releasePorts(endpoint)
	if err != nil {
		log.G(context.TODO()).Warn(err)
	}

	endpoint.portMapping = nil

	// Clean the connection tracker state of the host for the specific endpoint. This is a precautionary measure to
	// avoid new endpoints getting the same IP address to receive unexpected packets due to bad conntrack state leading
	// to bad NATing.
	clearConntrackEntries(d.nlh, endpoint)

	if err = d.storeUpdate(endpoint); err != nil {
		return fmt.Errorf("failed to update bridge endpoint %.7s to store: %v", endpoint.id, err)
	}

	return nil
}

func (d *driver) link(network *bridgeNetwork, endpoint *bridgeEndpoint, enable bool) (retErr error) {
	cc := endpoint.containerConfig
	ec := endpoint.extConnConfig
	if cc == nil || ec == nil || (len(cc.ParentEndpoints) == 0 && len(cc.ChildEndpoints) == 0) {
		// nothing to do
		return nil
	}

	// Try to keep things atomic. addedLinks keeps track of links that were
	// successfully added. If any error occurred, then roll back all.
	var addedLinks []*link
	defer func() {
		if retErr == nil {
			return
		}
		for _, l := range addedLinks {
			l.Disable()
		}
	}()

	if ec.ExposedPorts != nil {
		for _, p := range cc.ParentEndpoints {
			parentEndpoint, err := network.getEndpoint(p)
			if err != nil {
				return err
			}
			if parentEndpoint == nil {
				return InvalidEndpointIDError(p)
			}

			l, err := newLink(parentEndpoint.addr.IP, endpoint.addr.IP, ec.ExposedPorts, network.config.BridgeName)
			if err != nil {
				return err
			}
			if enable {
				if err := l.Enable(); err != nil {
					return err
				}
				addedLinks = append(addedLinks, l)
			} else {
				l.Disable()
			}
		}
	}

	for _, c := range cc.ChildEndpoints {
		childEndpoint, err := network.getEndpoint(c)
		if err != nil {
			return err
		}
		if childEndpoint == nil {
			return InvalidEndpointIDError(c)
		}
		if childEndpoint.extConnConfig == nil || childEndpoint.extConnConfig.ExposedPorts == nil {
			continue
		}

		l, err := newLink(endpoint.addr.IP, childEndpoint.addr.IP, childEndpoint.extConnConfig.ExposedPorts, network.config.BridgeName)
		if err != nil {
			return err
		}
		if enable {
			if err := l.Enable(); err != nil {
				return err
			}
			addedLinks = append(addedLinks, l)
		} else {
			l.Disable()
		}
	}

	return nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

func parseContainerOptions(cOptions map[string]interface{}) (*containerConfiguration, error) {
	if cOptions == nil {
		return nil, nil
	}
	genericData := cOptions[netlabel.GenericData]
	if genericData == nil {
		return nil, nil
	}
	switch opt := genericData.(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &containerConfiguration{})
		if err != nil {
			return nil, err
		}
		return opaqueConfig.(*containerConfiguration), nil
	case *containerConfiguration:
		return opt, nil
	default:
		return nil, nil
	}
}

func parseConnectivityOptions(cOptions map[string]interface{}) (*connectivityConfiguration, error) {
	if cOptions == nil {
		return nil, nil
	}

	cc := &connectivityConfiguration{}

	if opt, ok := cOptions[netlabel.PortMap]; ok {
		if pb, ok := opt.([]types.PortBinding); ok {
			cc.PortBindings = pb
		} else {
			return nil, types.InvalidParameterErrorf("invalid port mapping data in connectivity configuration: %v", opt)
		}
	}

	if opt, ok := cOptions[netlabel.ExposedPorts]; ok {
		if ports, ok := opt.([]types.TransportPort); ok {
			cc.ExposedPorts = ports
		} else {
			return nil, types.InvalidParameterErrorf("invalid exposed ports data in connectivity configuration: %v", opt)
		}
	}

	return cc, nil
}
