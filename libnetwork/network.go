package libnetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/etchosts"
	"github.com/docker/docker/libnetwork/internal/setmatrix"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/networkdb"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/stringid"
)

// EndpointWalker is a client provided function which will be used to walk the Endpoints.
// When the function returns true, the walk will stop.
type EndpointWalker func(ep *Endpoint) bool

// ipInfo is the reverse mapping from IP to service name to serve the PTR query.
// extResolver is set if an external server resolves a service name to this IP.
// It's an indication to defer PTR queries also to that external server.
type ipInfo struct {
	name        string
	serviceID   string
	extResolver bool
}

// svcMapEntry is the body of the element into the svcMap
// The ip is a string because the SetMatrix does not accept non hashable values
type svcMapEntry struct {
	ip        string
	serviceID string
}

type svcInfo struct {
	svcMap     setmatrix.SetMatrix[svcMapEntry]
	svcIPv6Map setmatrix.SetMatrix[svcMapEntry]
	ipMap      setmatrix.SetMatrix[ipInfo]
	service    map[string][]servicePorts
}

// backing container or host's info
type serviceTarget struct {
	name string
	ip   net.IP
	port uint16
}

type servicePorts struct {
	portName string
	proto    string
	target   []serviceTarget
}

type networkDBTable struct {
	name    string
	objType driverapi.ObjectType
}

// IpamConf contains all the ipam related configurations for a network
type IpamConf struct {
	// PreferredPool is the master address pool for containers and network interfaces.
	PreferredPool string
	// SubPool is a subset of the master pool. If specified,
	// this becomes the container pool.
	SubPool string
	// Gateway is the preferred Network Gateway address (optional).
	Gateway string
	// AuxAddresses contains auxiliary addresses for network driver. Must be within the master pool.
	// libnetwork will reserve them if they fall into the container pool.
	AuxAddresses map[string]string
}

// Validate checks whether the configuration is valid
func (c *IpamConf) Validate() error {
	if c.Gateway != "" && nil == net.ParseIP(c.Gateway) {
		return types.BadRequestErrorf("invalid gateway address %s in Ipam configuration", c.Gateway)
	}
	return nil
}

// IpamInfo contains all the ipam related operational info for a network
type IpamInfo struct {
	PoolID string
	Meta   map[string]string
	driverapi.IPAMData
}

// MarshalJSON encodes IpamInfo into json message
func (i *IpamInfo) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"PoolID": i.PoolID,
	}
	v, err := json.Marshal(&i.IPAMData)
	if err != nil {
		return nil, err
	}
	m["IPAMData"] = string(v)

	if i.Meta != nil {
		m["Meta"] = i.Meta
	}
	return json.Marshal(m)
}

// UnmarshalJSON decodes json message into PoolData
func (i *IpamInfo) UnmarshalJSON(data []byte) error {
	var (
		m   map[string]interface{}
		err error
	)
	if err = json.Unmarshal(data, &m); err != nil {
		return err
	}
	i.PoolID = m["PoolID"].(string)
	if v, ok := m["Meta"]; ok {
		b, _ := json.Marshal(v)
		if err = json.Unmarshal(b, &i.Meta); err != nil {
			return err
		}
	}
	if v, ok := m["IPAMData"]; ok {
		if err = json.Unmarshal([]byte(v.(string)), &i.IPAMData); err != nil {
			return err
		}
	}
	return nil
}

// Network represents a logical connectivity zone that containers may
// join using the Link method. A network is managed by a specific driver.
type Network struct {
	ctrlr            *Controller
	name             string
	networkType      string
	id               string
	created          time.Time
	scope            string // network data scope
	labels           map[string]string
	ipamType         string
	ipamOptions      map[string]string
	addrSpace        string
	ipamV4Config     []*IpamConf
	ipamV6Config     []*IpamConf
	ipamV4Info       []*IpamInfo
	ipamV6Info       []*IpamInfo
	enableIPv6       bool
	postIPv6         bool
	epCnt            *endpointCnt
	generic          options.Generic
	dbIndex          uint64
	dbExists         bool
	persist          bool
	drvOnce          *sync.Once
	resolverOnce     sync.Once //nolint:nolintlint,unused // only used on windows
	resolver         []*Resolver
	internal         bool
	attachable       bool
	inDelete         bool
	ingress          bool
	driverTables     []networkDBTable
	dynamic          bool
	configOnly       bool
	configFrom       string
	loadBalancerIP   net.IP
	loadBalancerMode string
	mu               sync.Mutex
}

const (
	loadBalancerModeNAT     = "NAT"
	loadBalancerModeDSR     = "DSR"
	loadBalancerModeDefault = loadBalancerModeNAT
)

// Name returns a user chosen name for this network.
func (n *Network) Name() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.name
}

// ID returns a system generated id for this network.
func (n *Network) ID() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.id
}

func (n *Network) Created() time.Time {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.created
}

// Type returns the type of network, which corresponds to its managing driver.
func (n *Network) Type() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.networkType
}

func (n *Network) Key() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return []string{datastore.NetworkKeyPrefix, n.id}
}

func (n *Network) KeyPrefix() []string {
	return []string{datastore.NetworkKeyPrefix}
}

func (n *Network) Value() []byte {
	n.mu.Lock()
	defer n.mu.Unlock()
	b, err := json.Marshal(n)
	if err != nil {
		return nil
	}
	return b
}

func (n *Network) SetValue(value []byte) error {
	return json.Unmarshal(value, n)
}

func (n *Network) Index() uint64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dbIndex
}

func (n *Network) SetIndex(index uint64) {
	n.mu.Lock()
	n.dbIndex = index
	n.dbExists = true
	n.mu.Unlock()
}

func (n *Network) Exists() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dbExists
}

func (n *Network) Skip() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return !n.persist
}

func (n *Network) New() datastore.KVObject {
	n.mu.Lock()
	defer n.mu.Unlock()

	return &Network{
		ctrlr:   n.ctrlr,
		drvOnce: &sync.Once{},
		scope:   n.scope,
	}
}

// CopyTo deep copies to the destination IpamConfig
func (c *IpamConf) CopyTo(dstC *IpamConf) error {
	dstC.PreferredPool = c.PreferredPool
	dstC.SubPool = c.SubPool
	dstC.Gateway = c.Gateway
	if c.AuxAddresses != nil {
		dstC.AuxAddresses = make(map[string]string, len(c.AuxAddresses))
		for k, v := range c.AuxAddresses {
			dstC.AuxAddresses[k] = v
		}
	}
	return nil
}

// CopyTo deep copies to the destination IpamInfo
func (i *IpamInfo) CopyTo(dstI *IpamInfo) error {
	dstI.PoolID = i.PoolID
	if i.Meta != nil {
		dstI.Meta = make(map[string]string)
		for k, v := range i.Meta {
			dstI.Meta[k] = v
		}
	}

	dstI.AddressSpace = i.AddressSpace
	dstI.Pool = types.GetIPNetCopy(i.Pool)
	dstI.Gateway = types.GetIPNetCopy(i.Gateway)

	if i.AuxAddresses != nil {
		dstI.AuxAddresses = make(map[string]*net.IPNet)
		for k, v := range i.AuxAddresses {
			dstI.AuxAddresses[k] = types.GetIPNetCopy(v)
		}
	}

	return nil
}

func (n *Network) validateConfiguration() error {
	if n.configOnly {
		// Only supports network specific configurations.
		// Network operator configurations are not supported.
		if n.ingress || n.internal || n.attachable || n.scope != "" {
			return types.ForbiddenErrorf("configuration network can only contain network " +
				"specific fields. Network operator fields like " +
				"[ ingress | internal | attachable | scope ] are not supported.")
		}
	}
	if n.configFrom != "" {
		if n.configOnly {
			return types.ForbiddenErrorf("a configuration network cannot depend on another configuration network")
		}
		if n.ipamType != "" &&
			n.ipamType != defaultIpamForNetworkType(n.networkType) ||
			n.enableIPv6 ||
			len(n.labels) > 0 || len(n.ipamOptions) > 0 ||
			len(n.ipamV4Config) > 0 || len(n.ipamV6Config) > 0 {
			return types.ForbiddenErrorf("user specified configurations are not supported if the network depends on a configuration network")
		}
		if len(n.generic) > 0 {
			if data, ok := n.generic[netlabel.GenericData]; ok {
				var (
					driverOptions map[string]string
					opts          interface{}
				)
				switch t := data.(type) {
				case map[string]interface{}, map[string]string:
					opts = t
				}
				ba, err := json.Marshal(opts)
				if err != nil {
					return fmt.Errorf("failed to validate network configuration: %v", err)
				}
				if err := json.Unmarshal(ba, &driverOptions); err != nil {
					return fmt.Errorf("failed to validate network configuration: %v", err)
				}
				if len(driverOptions) > 0 {
					return types.ForbiddenErrorf("network driver options are not supported if the network depends on a configuration network")
				}
			}
		}
	}
	return nil
}

// applyConfigurationTo applies network specific configurations.
func (n *Network) applyConfigurationTo(to *Network) error {
	to.enableIPv6 = n.enableIPv6
	if len(n.labels) > 0 {
		to.labels = make(map[string]string, len(n.labels))
		for k, v := range n.labels {
			if _, ok := to.labels[k]; !ok {
				to.labels[k] = v
			}
		}
	}
	if len(n.ipamType) != 0 {
		to.ipamType = n.ipamType
	}
	if len(n.ipamOptions) > 0 {
		to.ipamOptions = make(map[string]string, len(n.ipamOptions))
		for k, v := range n.ipamOptions {
			if _, ok := to.ipamOptions[k]; !ok {
				to.ipamOptions[k] = v
			}
		}
	}
	if len(n.ipamV4Config) > 0 {
		to.ipamV4Config = make([]*IpamConf, 0, len(n.ipamV4Config))
		to.ipamV4Config = append(to.ipamV4Config, n.ipamV4Config...)
	}
	if len(n.ipamV6Config) > 0 {
		to.ipamV6Config = make([]*IpamConf, 0, len(n.ipamV6Config))
		to.ipamV6Config = append(to.ipamV6Config, n.ipamV6Config...)
	}
	if len(n.generic) > 0 {
		to.generic = options.Generic{}
		for k, v := range n.generic {
			to.generic[k] = v
		}
	}
	return nil
}

func (n *Network) CopyTo(o datastore.KVObject) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	dstN := o.(*Network)
	dstN.name = n.name
	dstN.id = n.id
	dstN.created = n.created
	dstN.networkType = n.networkType
	dstN.scope = n.scope
	dstN.dynamic = n.dynamic
	dstN.ipamType = n.ipamType
	dstN.enableIPv6 = n.enableIPv6
	dstN.persist = n.persist
	dstN.postIPv6 = n.postIPv6
	dstN.dbIndex = n.dbIndex
	dstN.dbExists = n.dbExists
	dstN.drvOnce = n.drvOnce
	dstN.internal = n.internal
	dstN.attachable = n.attachable
	dstN.inDelete = n.inDelete
	dstN.ingress = n.ingress
	dstN.configOnly = n.configOnly
	dstN.configFrom = n.configFrom
	dstN.loadBalancerIP = n.loadBalancerIP
	dstN.loadBalancerMode = n.loadBalancerMode

	// copy labels
	if dstN.labels == nil {
		dstN.labels = make(map[string]string, len(n.labels))
	}
	for k, v := range n.labels {
		dstN.labels[k] = v
	}

	if n.ipamOptions != nil {
		dstN.ipamOptions = make(map[string]string, len(n.ipamOptions))
		for k, v := range n.ipamOptions {
			dstN.ipamOptions[k] = v
		}
	}

	for _, v4conf := range n.ipamV4Config {
		dstV4Conf := &IpamConf{}
		if err := v4conf.CopyTo(dstV4Conf); err != nil {
			return err
		}
		dstN.ipamV4Config = append(dstN.ipamV4Config, dstV4Conf)
	}

	for _, v4info := range n.ipamV4Info {
		dstV4Info := &IpamInfo{}
		if err := v4info.CopyTo(dstV4Info); err != nil {
			return err
		}
		dstN.ipamV4Info = append(dstN.ipamV4Info, dstV4Info)
	}

	for _, v6conf := range n.ipamV6Config {
		dstV6Conf := &IpamConf{}
		if err := v6conf.CopyTo(dstV6Conf); err != nil {
			return err
		}
		dstN.ipamV6Config = append(dstN.ipamV6Config, dstV6Conf)
	}

	for _, v6info := range n.ipamV6Info {
		dstV6Info := &IpamInfo{}
		if err := v6info.CopyTo(dstV6Info); err != nil {
			return err
		}
		dstN.ipamV6Info = append(dstN.ipamV6Info, dstV6Info)
	}

	dstN.generic = options.Generic{}
	for k, v := range n.generic {
		dstN.generic[k] = v
	}

	return nil
}

func (n *Network) DataScope() string {
	s := n.Scope()
	// All swarm scope networks have local datascope
	if s == scope.Swarm {
		s = scope.Local
	}
	return s
}

func (n *Network) getEpCnt() *endpointCnt {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.epCnt
}

// TODO : Can be made much more generic with the help of reflection (but has some golang limitations)
func (n *Network) MarshalJSON() ([]byte, error) {
	netMap := make(map[string]interface{})
	netMap["name"] = n.name
	netMap["id"] = n.id
	netMap["created"] = n.created
	netMap["networkType"] = n.networkType
	netMap["scope"] = n.scope
	netMap["labels"] = n.labels
	netMap["ipamType"] = n.ipamType
	netMap["ipamOptions"] = n.ipamOptions
	netMap["addrSpace"] = n.addrSpace
	netMap["enableIPv6"] = n.enableIPv6
	if n.generic != nil {
		netMap["generic"] = n.generic
	}
	netMap["persist"] = n.persist
	netMap["postIPv6"] = n.postIPv6
	if len(n.ipamV4Config) > 0 {
		ics, err := json.Marshal(n.ipamV4Config)
		if err != nil {
			return nil, err
		}
		netMap["ipamV4Config"] = string(ics)
	}
	if len(n.ipamV4Info) > 0 {
		iis, err := json.Marshal(n.ipamV4Info)
		if err != nil {
			return nil, err
		}
		netMap["ipamV4Info"] = string(iis)
	}
	if len(n.ipamV6Config) > 0 {
		ics, err := json.Marshal(n.ipamV6Config)
		if err != nil {
			return nil, err
		}
		netMap["ipamV6Config"] = string(ics)
	}
	if len(n.ipamV6Info) > 0 {
		iis, err := json.Marshal(n.ipamV6Info)
		if err != nil {
			return nil, err
		}
		netMap["ipamV6Info"] = string(iis)
	}
	netMap["internal"] = n.internal
	netMap["attachable"] = n.attachable
	netMap["inDelete"] = n.inDelete
	netMap["ingress"] = n.ingress
	netMap["configOnly"] = n.configOnly
	netMap["configFrom"] = n.configFrom
	netMap["loadBalancerIP"] = n.loadBalancerIP
	netMap["loadBalancerMode"] = n.loadBalancerMode
	return json.Marshal(netMap)
}

// TODO : Can be made much more generic with the help of reflection (but has some golang limitations)
func (n *Network) UnmarshalJSON(b []byte) (err error) {
	var netMap map[string]interface{}
	if err := json.Unmarshal(b, &netMap); err != nil {
		return err
	}
	n.name = netMap["name"].(string)
	n.id = netMap["id"].(string)
	// "created" is not available in older versions
	if v, ok := netMap["created"]; ok {
		// n.created is time.Time but marshalled as string
		if err = n.created.UnmarshalText([]byte(v.(string))); err != nil {
			log.G(context.TODO()).Warnf("failed to unmarshal creation time %v: %v", v, err)
			n.created = time.Time{}
		}
	}
	n.networkType = netMap["networkType"].(string)
	n.enableIPv6 = netMap["enableIPv6"].(bool)

	// if we weren't unmarshaling to netMap we could simply set n.labels
	// unfortunately, we can't because map[string]interface{} != map[string]string
	if labels, ok := netMap["labels"].(map[string]interface{}); ok {
		n.labels = make(map[string]string, len(labels))
		for label, value := range labels {
			n.labels[label] = value.(string)
		}
	}

	if v, ok := netMap["ipamOptions"]; ok {
		if iOpts, ok := v.(map[string]interface{}); ok {
			n.ipamOptions = make(map[string]string, len(iOpts))
			for k, v := range iOpts {
				n.ipamOptions[k] = v.(string)
			}
		}
	}

	if v, ok := netMap["generic"]; ok {
		n.generic = v.(map[string]interface{})
		// Restore opts in their map[string]string form
		if v, ok := n.generic[netlabel.GenericData]; ok {
			var lmap map[string]string
			ba, err := json.Marshal(v)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(ba, &lmap); err != nil {
				return err
			}
			n.generic[netlabel.GenericData] = lmap
		}
	}
	if v, ok := netMap["persist"]; ok {
		n.persist = v.(bool)
	}
	if v, ok := netMap["postIPv6"]; ok {
		n.postIPv6 = v.(bool)
	}
	if v, ok := netMap["ipamType"]; ok {
		n.ipamType = v.(string)
	} else {
		n.ipamType = ipamapi.DefaultIPAM
	}
	if v, ok := netMap["addrSpace"]; ok {
		n.addrSpace = v.(string)
	}
	if v, ok := netMap["ipamV4Config"]; ok {
		if err := json.Unmarshal([]byte(v.(string)), &n.ipamV4Config); err != nil {
			return err
		}
	}
	if v, ok := netMap["ipamV4Info"]; ok {
		if err := json.Unmarshal([]byte(v.(string)), &n.ipamV4Info); err != nil {
			return err
		}
	}
	if v, ok := netMap["ipamV6Config"]; ok {
		if err := json.Unmarshal([]byte(v.(string)), &n.ipamV6Config); err != nil {
			return err
		}
	}
	if v, ok := netMap["ipamV6Info"]; ok {
		if err := json.Unmarshal([]byte(v.(string)), &n.ipamV6Info); err != nil {
			return err
		}
	}
	if v, ok := netMap["internal"]; ok {
		n.internal = v.(bool)
	}
	if v, ok := netMap["attachable"]; ok {
		n.attachable = v.(bool)
	}
	if s, ok := netMap["scope"]; ok {
		n.scope = s.(string)
	}
	if v, ok := netMap["inDelete"]; ok {
		n.inDelete = v.(bool)
	}
	if v, ok := netMap["ingress"]; ok {
		n.ingress = v.(bool)
	}
	if v, ok := netMap["configOnly"]; ok {
		n.configOnly = v.(bool)
	}
	if v, ok := netMap["configFrom"]; ok {
		n.configFrom = v.(string)
	}
	if v, ok := netMap["loadBalancerIP"]; ok {
		n.loadBalancerIP = net.ParseIP(v.(string))
	}
	n.loadBalancerMode = loadBalancerModeDefault
	if v, ok := netMap["loadBalancerMode"]; ok {
		n.loadBalancerMode = v.(string)
	}
	// Reconcile old networks with the recently added `--ipv6` flag
	if !n.enableIPv6 {
		n.enableIPv6 = len(n.ipamV6Info) > 0
	}
	return nil
}

// NetworkOption is an option setter function type used to pass various options to
// NewNetwork method. The various setter functions of type NetworkOption are
// provided by libnetwork, they look like NetworkOptionXXXX(...)
type NetworkOption func(n *Network)

// NetworkOptionGeneric function returns an option setter for a Generic option defined
// in a Dictionary of Key-Value pair
func NetworkOptionGeneric(generic map[string]interface{}) NetworkOption {
	return func(n *Network) {
		if n.generic == nil {
			n.generic = make(map[string]interface{})
		}
		if val, ok := generic[netlabel.EnableIPv6]; ok {
			n.enableIPv6 = val.(bool)
		}
		if val, ok := generic[netlabel.Internal]; ok {
			n.internal = val.(bool)
		}
		for k, v := range generic {
			n.generic[k] = v
		}
	}
}

// NetworkOptionIngress returns an option setter to indicate if a network is
// an ingress network.
func NetworkOptionIngress(ingress bool) NetworkOption {
	return func(n *Network) {
		n.ingress = ingress
	}
}

// NetworkOptionPersist returns an option setter to set persistence policy for a network
func NetworkOptionPersist(persist bool) NetworkOption {
	return func(n *Network) {
		n.persist = persist
	}
}

// NetworkOptionEnableIPv6 returns an option setter to explicitly configure IPv6
func NetworkOptionEnableIPv6(enableIPv6 bool) NetworkOption {
	return func(n *Network) {
		if n.generic == nil {
			n.generic = make(map[string]interface{})
		}
		n.enableIPv6 = enableIPv6
		n.generic[netlabel.EnableIPv6] = enableIPv6
	}
}

// NetworkOptionInternalNetwork returns an option setter to config the network
// to be internal which disables default gateway service
func NetworkOptionInternalNetwork() NetworkOption {
	return func(n *Network) {
		if n.generic == nil {
			n.generic = make(map[string]interface{})
		}
		n.internal = true
		n.generic[netlabel.Internal] = true
	}
}

// NetworkOptionAttachable returns an option setter to set attachable for a network
func NetworkOptionAttachable(attachable bool) NetworkOption {
	return func(n *Network) {
		n.attachable = attachable
	}
}

// NetworkOptionScope returns an option setter to overwrite the network's scope.
// By default the network's scope is set to the network driver's datascope.
func NetworkOptionScope(scope string) NetworkOption {
	return func(n *Network) {
		n.scope = scope
	}
}

// NetworkOptionIpam function returns an option setter for the ipam configuration for this network
func NetworkOptionIpam(ipamDriver string, addrSpace string, ipV4 []*IpamConf, ipV6 []*IpamConf, opts map[string]string) NetworkOption {
	return func(n *Network) {
		if ipamDriver != "" {
			n.ipamType = ipamDriver
			if ipamDriver == ipamapi.DefaultIPAM {
				n.ipamType = defaultIpamForNetworkType(n.Type())
			}
		}
		n.ipamOptions = opts
		n.addrSpace = addrSpace
		n.ipamV4Config = ipV4
		n.ipamV6Config = ipV6
	}
}

// NetworkOptionLBEndpoint function returns an option setter for the configuration of the load balancer endpoint for this network
func NetworkOptionLBEndpoint(ip net.IP) NetworkOption {
	return func(n *Network) {
		n.loadBalancerIP = ip
	}
}

// NetworkOptionDriverOpts function returns an option setter for any driver parameter described by a map
func NetworkOptionDriverOpts(opts map[string]string) NetworkOption {
	return func(n *Network) {
		if n.generic == nil {
			n.generic = make(map[string]interface{})
		}
		if opts == nil {
			opts = make(map[string]string)
		}
		// Store the options
		n.generic[netlabel.GenericData] = opts
	}
}

// NetworkOptionLabels function returns an option setter for labels specific to a network
func NetworkOptionLabels(labels map[string]string) NetworkOption {
	return func(n *Network) {
		n.labels = labels
	}
}

// NetworkOptionDynamic function returns an option setter for dynamic option for a network
func NetworkOptionDynamic() NetworkOption {
	return func(n *Network) {
		n.dynamic = true
	}
}

// NetworkOptionDeferIPv6Alloc instructs the network to defer the IPV6 address allocation until after the endpoint has been created
// It is being provided to support the specific docker daemon flags where user can deterministically assign an IPv6 address
// to a container as combination of fixed-cidr-v6 + mac-address
// TODO: Remove this option setter once we support endpoint ipam options
func NetworkOptionDeferIPv6Alloc(enable bool) NetworkOption {
	return func(n *Network) {
		n.postIPv6 = enable
	}
}

// NetworkOptionConfigOnly tells controller this network is
// a configuration only network. It serves as a configuration
// for other networks.
func NetworkOptionConfigOnly() NetworkOption {
	return func(n *Network) {
		n.configOnly = true
	}
}

// NetworkOptionConfigFrom tells controller to pick the
// network configuration from a configuration only network
func NetworkOptionConfigFrom(name string) NetworkOption {
	return func(n *Network) {
		n.configFrom = name
	}
}

func (n *Network) processOptions(options ...NetworkOption) {
	for _, opt := range options {
		if opt != nil {
			opt(n)
		}
	}
}

type networkDeleteParams struct {
	rmLBEndpoint bool
}

// NetworkDeleteOption is a type for optional parameters to pass to the
// Network.Delete() function.
type NetworkDeleteOption func(p *networkDeleteParams)

// NetworkDeleteOptionRemoveLB informs a Network.Delete() operation that should
// remove the load balancer endpoint for this network.  Note that the Delete()
// method will automatically remove a load balancing endpoint for most networks
// when the network is otherwise empty.  However, this does not occur for some
// networks.  In particular, networks marked as ingress (which are supposed to
// be more permanent than other overlay networks) won't automatically remove
// the LB endpoint on Delete().  This method allows for explicit removal of
// such networks provided there are no other endpoints present in the network.
// If the network still has non-LB endpoints present, Delete() will not
// remove the LB endpoint and will return an error.
func NetworkDeleteOptionRemoveLB(p *networkDeleteParams) {
	p.rmLBEndpoint = true
}

func (n *Network) resolveDriver(name string, load bool) (driverapi.Driver, driverapi.Capability, error) {
	c := n.getController()

	// Check if a driver for the specified network type is available
	d, capabilities := c.drvRegistry.Driver(name)
	if d == nil {
		if load {
			err := c.loadDriver(name)
			if err != nil {
				return nil, driverapi.Capability{}, err
			}

			d, capabilities = c.drvRegistry.Driver(name)
			if d == nil {
				return nil, driverapi.Capability{}, fmt.Errorf("could not resolve driver %s in registry", name)
			}
		} else {
			// don't fail if driver loading is not required
			return nil, driverapi.Capability{}, nil
		}
	}

	return d, capabilities, nil
}

func (n *Network) driverIsMultihost() bool {
	_, capabilities, err := n.resolveDriver(n.networkType, true)
	if err != nil {
		return false
	}
	return capabilities.ConnectivityScope == scope.Global
}

func (n *Network) driver(load bool) (driverapi.Driver, error) {
	d, capabilities, err := n.resolveDriver(n.networkType, load)
	if err != nil {
		return nil, err
	}

	n.mu.Lock()
	// If load is not required, driver, cap and err may all be nil
	if n.scope == "" {
		n.scope = capabilities.DataScope
	}
	if n.dynamic {
		// If the network is dynamic, then it is swarm
		// scoped regardless of the backing driver.
		n.scope = scope.Swarm
	}
	n.mu.Unlock()
	return d, nil
}

// Delete the network.
func (n *Network) Delete(options ...NetworkDeleteOption) error {
	var params networkDeleteParams
	for _, opt := range options {
		opt(&params)
	}
	return n.delete(false, params.rmLBEndpoint)
}

// This function gets called in 3 ways:
//   - Delete() -- (false, false)
//     remove if endpoint count == 0 or endpoint count == 1 and
//     there is a load balancer IP
//   - Delete(libnetwork.NetworkDeleteOptionRemoveLB) -- (false, true)
//     remove load balancer and network if endpoint count == 1
//   - controller.networkCleanup() -- (true, true)
//     remove the network no matter what
func (n *Network) delete(force bool, rmLBEndpoint bool) error {
	n.mu.Lock()
	c := n.ctrlr
	name := n.name
	id := n.id
	n.mu.Unlock()

	c.networkLocker.Lock(id)
	defer c.networkLocker.Unlock(id) //nolint:errcheck

	n, err := c.getNetworkFromStore(id)
	if err != nil {
		return &UnknownNetworkError{name: name, id: id}
	}

	// Only remove ingress on force removal or explicit LB endpoint removal
	if n.ingress && !force && !rmLBEndpoint {
		return &ActiveEndpointsError{name: n.name, id: n.id}
	}

	// Check that the network is empty
	var emptyCount uint64
	if n.hasLoadBalancerEndpoint() {
		emptyCount = 1
	}
	if !force && n.getEpCnt().EndpointCnt() > emptyCount {
		if n.configOnly {
			return types.ForbiddenErrorf("configuration network %q is in use", n.Name())
		}
		return &ActiveEndpointsError{name: n.name, id: n.id}
	}

	if n.hasLoadBalancerEndpoint() {
		// If we got to this point, then the following must hold:
		//  * force is true OR endpoint count == 1
		if err := n.deleteLoadBalancerSandbox(); err != nil {
			if !force {
				return err
			}
			// continue deletion when force is true even on error
			log.G(context.TODO()).Warnf("Error deleting load balancer sandbox: %v", err)
		}
		// Reload the network from the store to update the epcnt.
		n, err = c.getNetworkFromStore(id)
		if err != nil {
			return &UnknownNetworkError{name: name, id: id}
		}
	}

	// Up to this point, errors that we returned were recoverable.
	// From here on, any errors leave us in an inconsistent state.
	// This is unfortunate, but there isn't a safe way to
	// reconstitute a load-balancer endpoint after removing it.

	// Mark the network for deletion
	n.inDelete = true
	if err = c.updateToStore(n); err != nil {
		return fmt.Errorf("error marking network %s (%s) for deletion: %v", n.Name(), n.ID(), err)
	}

	if n.ConfigFrom() != "" {
		if t, err := c.getConfigNetwork(n.ConfigFrom()); err == nil {
			if err := t.getEpCnt().DecEndpointCnt(); err != nil {
				log.G(context.TODO()).Warnf("Failed to update reference count for configuration network %q on removal of network %q: %v",
					t.Name(), n.Name(), err)
			}
		} else {
			log.G(context.TODO()).Warnf("Could not find configuration network %q during removal of network %q", n.configFrom, n.Name())
		}
	}

	if n.configOnly {
		goto removeFromStore
	}

	n.ipamRelease()

	// We are about to delete the network. Leave the gossip
	// cluster for the network to stop all incoming network
	// specific gossip updates before cleaning up all the service
	// bindings for the network. But cleanup service binding
	// before deleting the network from the store since service
	// bindings cleanup requires the network in the store.
	n.cancelDriverWatches()
	if err = n.leaveCluster(); err != nil {
		log.G(context.TODO()).Errorf("Failed leaving network %s from the agent cluster: %v", n.Name(), err)
	}

	// Cleanup the service discovery for this network
	c.cleanupServiceDiscovery(n.ID())

	// Cleanup the load balancer. On Windows this call is required
	// to remove remote loadbalancers in VFP, and must be performed before
	// dataplane network deletion.
	if runtime.GOOS == "windows" {
		c.cleanupServiceBindings(n.ID())
	}

	// Delete the network from the dataplane
	if err = n.deleteNetwork(); err != nil {
		if !force {
			return err
		}
		log.G(context.TODO()).Debugf("driver failed to delete stale network %s (%s): %v", n.Name(), n.ID(), err)
	}

removeFromStore:
	// deleteFromStore performs an atomic delete operation and the
	// Network.epCnt will help prevent any possible
	// race between endpoint join and network delete
	if err = c.deleteFromStore(n.getEpCnt()); err != nil {
		if !force {
			return fmt.Errorf("error deleting network endpoint count from store: %v", err)
		}
		log.G(context.TODO()).Debugf("Error deleting endpoint count from store for stale network %s (%s) for deletion: %v", n.Name(), n.ID(), err)
	}

	if err = c.deleteFromStore(n); err != nil {
		return fmt.Errorf("error deleting network from store: %v", err)
	}

	return nil
}

func (n *Network) deleteNetwork() error {
	d, err := n.driver(true)
	if err != nil {
		return fmt.Errorf("failed deleting Network: %v", err)
	}

	if err := d.DeleteNetwork(n.ID()); err != nil {
		// Forbidden Errors should be honored
		if _, ok := err.(types.ForbiddenError); ok {
			return err
		}

		if _, ok := err.(types.MaskableError); !ok {
			log.G(context.TODO()).Warnf("driver error deleting network %s : %v", n.name, err)
		}
	}

	for _, resolver := range n.resolver {
		resolver.Stop()
	}
	return nil
}

func (n *Network) addEndpoint(ep *Endpoint) error {
	d, err := n.driver(true)
	if err != nil {
		return fmt.Errorf("failed to add endpoint: %v", err)
	}

	err = d.CreateEndpoint(n.id, ep.id, ep.Interface(), ep.generic)
	if err != nil {
		return types.InternalErrorf("failed to create endpoint %s on network %s: %v",
			ep.Name(), n.Name(), err)
	}

	return nil
}

// CreateEndpoint creates a new endpoint to this network symbolically identified by the
// specified unique name. The options parameter carries driver specific options.
func (n *Network) CreateEndpoint(name string, options ...EndpointOption) (*Endpoint, error) {
	var err error
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidName(name)
	}

	if n.ConfigOnly() {
		return nil, types.ForbiddenErrorf("cannot create endpoint on configuration-only network")
	}

	if _, err = n.EndpointByName(name); err == nil {
		return nil, types.ForbiddenErrorf("endpoint with name %s already exists in network %s", name, n.Name())
	}

	n.ctrlr.networkLocker.Lock(n.id)
	defer n.ctrlr.networkLocker.Unlock(n.id) //nolint:errcheck

	return n.createEndpoint(name, options...)
}

func (n *Network) createEndpoint(name string, options ...EndpointOption) (*Endpoint, error) {
	var err error

	ep := &Endpoint{name: name, generic: make(map[string]interface{}), iface: &endpointInterface{}}
	ep.id = stringid.GenerateRandomID()

	// Initialize ep.network with a possibly stale copy of n. We need this to get network from
	// store. But once we get it from store we will have the most uptodate copy possibly.
	ep.network = n
	ep.network, err = ep.getNetworkFromStore()
	if err != nil {
		log.G(context.TODO()).Errorf("failed to get network during CreateEndpoint: %v", err)
		return nil, err
	}
	n = ep.network

	ep.processOptions(options...)

	for _, llIPNet := range ep.Iface().LinkLocalAddresses() {
		if !llIPNet.IP.IsLinkLocalUnicast() {
			return nil, types.BadRequestErrorf("invalid link local IP address: %v", llIPNet.IP)
		}
	}

	if opt, ok := ep.generic[netlabel.MacAddress]; ok {
		if mac, ok := opt.(net.HardwareAddr); ok {
			ep.iface.mac = mac
		}
	}

	ipam, capability, err := n.getController().getIPAMDriver(n.ipamType)
	if err != nil {
		return nil, err
	}

	if capability.RequiresMACAddress {
		if ep.iface.mac == nil {
			ep.iface.mac = netutils.GenerateRandomMAC()
		}
		if ep.ipamOptions == nil {
			ep.ipamOptions = make(map[string]string)
		}
		ep.ipamOptions[netlabel.MacAddress] = ep.iface.mac.String()
	}

	if err = ep.assignAddress(ipam, true, n.enableIPv6 && !n.postIPv6); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			ep.releaseAddress()
		}
	}()

	if err = n.addEndpoint(ep); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if e := ep.deleteEndpoint(false); e != nil {
				log.G(context.TODO()).Warnf("cleaning up endpoint failed %s : %v", name, e)
			}
		}
	}()

	// We should perform updateToStore call right after addEndpoint
	// in order to have iface properly configured
	if err = n.getController().updateToStore(ep); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if e := n.getController().deleteFromStore(ep); e != nil {
				log.G(context.TODO()).Warnf("error rolling back endpoint %s from store: %v", name, e)
			}
		}
	}()

	if err = ep.assignAddress(ipam, false, n.enableIPv6 && n.postIPv6); err != nil {
		return nil, err
	}

	// Watch for service records
	n.getController().watchSvcRecord(ep)
	defer func() {
		if err != nil {
			n.getController().unWatchSvcRecord(ep)
		}
	}()

	// Increment endpoint count to indicate completion of endpoint addition
	if err = n.getEpCnt().IncEndpointCnt(); err != nil {
		return nil, err
	}

	return ep, nil
}

// Endpoints returns the list of Endpoint(s) in this network.
func (n *Network) Endpoints() []*Endpoint {
	endpoints, err := n.getEndpointsFromStore()
	if err != nil {
		log.G(context.TODO()).Error(err)
	}
	return endpoints
}

// WalkEndpoints uses the provided function to walk the Endpoints.
func (n *Network) WalkEndpoints(walker EndpointWalker) {
	for _, e := range n.Endpoints() {
		if walker(e) {
			return
		}
	}
}

// EndpointByName returns the Endpoint which has the passed name. If not found,
// the error ErrNoSuchEndpoint is returned.
func (n *Network) EndpointByName(name string) (*Endpoint, error) {
	if name == "" {
		return nil, ErrInvalidName(name)
	}
	var e *Endpoint

	s := func(current *Endpoint) bool {
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

// EndpointByID returns the Endpoint which has the passed id. If not found,
// the error ErrNoSuchEndpoint is returned.
func (n *Network) EndpointByID(id string) (*Endpoint, error) {
	if id == "" {
		return nil, ErrInvalidID(id)
	}

	ep, err := n.getEndpointFromStore(id)
	if err != nil {
		return nil, ErrNoSuchEndpoint(id)
	}

	return ep, nil
}

func (n *Network) updateSvcRecord(ep *Endpoint, localEps []*Endpoint, isAdd bool) {
	var ipv6 net.IP
	epName := ep.Name()
	if iface := ep.Iface(); iface != nil && iface.Address() != nil {
		myAliases := ep.MyAliases()
		if iface.AddressIPv6() != nil {
			ipv6 = iface.AddressIPv6().IP
		}

		serviceID := ep.svcID
		if serviceID == "" {
			serviceID = ep.ID()
		}
		if isAdd {
			// If anonymous endpoint has an alias use the first alias
			// for ip->name mapping. Not having the reverse mapping
			// breaks some apps
			if ep.isAnonymous() {
				if len(myAliases) > 0 {
					n.addSvcRecords(ep.ID(), myAliases[0], serviceID, iface.Address().IP, ipv6, true, "updateSvcRecord")
				}
			} else {
				n.addSvcRecords(ep.ID(), epName, serviceID, iface.Address().IP, ipv6, true, "updateSvcRecord")
			}
			for _, alias := range myAliases {
				n.addSvcRecords(ep.ID(), alias, serviceID, iface.Address().IP, ipv6, false, "updateSvcRecord")
			}
		} else {
			if ep.isAnonymous() {
				if len(myAliases) > 0 {
					n.deleteSvcRecords(ep.ID(), myAliases[0], serviceID, iface.Address().IP, ipv6, true, "updateSvcRecord")
				}
			} else {
				n.deleteSvcRecords(ep.ID(), epName, serviceID, iface.Address().IP, ipv6, true, "updateSvcRecord")
			}
			for _, alias := range myAliases {
				n.deleteSvcRecords(ep.ID(), alias, serviceID, iface.Address().IP, ipv6, false, "updateSvcRecord")
			}
		}
	}
}

func addIPToName(ipMap *setmatrix.SetMatrix[ipInfo], name, serviceID string, ip net.IP) {
	reverseIP := netutils.ReverseIP(ip.String())
	ipMap.Insert(reverseIP, ipInfo{
		name:      name,
		serviceID: serviceID,
	})
}

func delIPToName(ipMap *setmatrix.SetMatrix[ipInfo], name, serviceID string, ip net.IP) {
	reverseIP := netutils.ReverseIP(ip.String())
	ipMap.Remove(reverseIP, ipInfo{
		name:      name,
		serviceID: serviceID,
	})
}

func addNameToIP(svcMap *setmatrix.SetMatrix[svcMapEntry], name, serviceID string, epIP net.IP) {
	// Since DNS name resolution is case-insensitive, Use the lower-case form
	// of the name as the key into svcMap
	lowerCaseName := strings.ToLower(name)
	svcMap.Insert(lowerCaseName, svcMapEntry{
		ip:        epIP.String(),
		serviceID: serviceID,
	})
}

func delNameToIP(svcMap *setmatrix.SetMatrix[svcMapEntry], name, serviceID string, epIP net.IP) {
	lowerCaseName := strings.ToLower(name)
	svcMap.Remove(lowerCaseName, svcMapEntry{
		ip:        epIP.String(),
		serviceID: serviceID,
	})
}

func (n *Network) addSvcRecords(eID, name, serviceID string, epIP, epIPv6 net.IP, ipMapUpdate bool, method string) {
	// Do not add service names for ingress network as this is a
	// routing only network
	if n.ingress {
		return
	}
	networkID := n.ID()
	log.G(context.TODO()).Debugf("%s (%.7s).addSvcRecords(%s, %s, %s, %t) %s sid:%s", eID, networkID, name, epIP, epIPv6, ipMapUpdate, method, serviceID)

	c := n.getController()
	c.mu.Lock()
	defer c.mu.Unlock()

	sr, ok := c.svcRecords[networkID]
	if !ok {
		sr = &svcInfo{}
		c.svcRecords[networkID] = sr
	}

	if ipMapUpdate {
		addIPToName(&sr.ipMap, name, serviceID, epIP)
		if epIPv6 != nil {
			addIPToName(&sr.ipMap, name, serviceID, epIPv6)
		}
	}

	addNameToIP(&sr.svcMap, name, serviceID, epIP)
	if epIPv6 != nil {
		addNameToIP(&sr.svcIPv6Map, name, serviceID, epIPv6)
	}
}

func (n *Network) deleteSvcRecords(eID, name, serviceID string, epIP net.IP, epIPv6 net.IP, ipMapUpdate bool, method string) {
	// Do not delete service names from ingress network as this is a
	// routing only network
	if n.ingress {
		return
	}
	networkID := n.ID()
	log.G(context.TODO()).Debugf("%s (%.7s).deleteSvcRecords(%s, %s, %s, %t) %s sid:%s ", eID, networkID, name, epIP, epIPv6, ipMapUpdate, method, serviceID)

	c := n.getController()
	c.mu.Lock()
	defer c.mu.Unlock()

	sr, ok := c.svcRecords[networkID]
	if !ok {
		return
	}

	if ipMapUpdate {
		delIPToName(&sr.ipMap, name, serviceID, epIP)

		if epIPv6 != nil {
			delIPToName(&sr.ipMap, name, serviceID, epIPv6)
		}
	}

	delNameToIP(&sr.svcMap, name, serviceID, epIP)

	if epIPv6 != nil {
		delNameToIP(&sr.svcIPv6Map, name, serviceID, epIPv6)
	}
}

func (n *Network) getSvcRecords(ep *Endpoint) []etchosts.Record {
	n.mu.Lock()
	defer n.mu.Unlock()

	if ep == nil {
		return nil
	}

	var recs []etchosts.Record

	epName := ep.Name()

	n.ctrlr.mu.Lock()
	defer n.ctrlr.mu.Unlock()
	sr, ok := n.ctrlr.svcRecords[n.id]
	if !ok {
		return nil
	}

	svcMapKeys := sr.svcMap.Keys()
	// Loop on service names on this network
	for _, k := range svcMapKeys {
		if strings.Split(k, ".")[0] == epName {
			continue
		}
		// Get all the IPs associated to this service
		mapEntryList, ok := sr.svcMap.Get(k)
		if !ok {
			// The key got deleted
			continue
		}
		if len(mapEntryList) == 0 {
			log.G(context.TODO()).Warnf("Found empty list of IP addresses for service %s on network %s (%s)", k, n.name, n.id)
			continue
		}

		recs = append(recs, etchosts.Record{
			Hosts: k,
			IP:    mapEntryList[0].ip,
		})
	}

	return recs
}

func (n *Network) getController() *Controller {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.ctrlr
}

func (n *Network) ipamAllocate() error {
	if n.hasSpecialDriver() {
		return nil
	}

	ipam, _, err := n.getController().getIPAMDriver(n.ipamType)
	if err != nil {
		return err
	}

	if n.addrSpace == "" {
		if n.addrSpace, err = n.deriveAddressSpace(); err != nil {
			return err
		}
	}

	err = n.ipamAllocateVersion(4, ipam)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			n.ipamReleaseVersion(4, ipam)
		}
	}()

	if !n.enableIPv6 {
		return nil
	}

	err = n.ipamAllocateVersion(6, ipam)
	return err
}

func (n *Network) requestPoolHelper(ipam ipamapi.Ipam, addressSpace, requestedPool, requestedSubPool string, options map[string]string, v6 bool) (poolID string, pool *net.IPNet, meta map[string]string, err error) {
	var tmpPoolLeases []string
	defer func() {
		// Prevent repeated lock/unlock in the loop.
		nwName := n.Name()
		// Release all pools we held on to.
		for _, pID := range tmpPoolLeases {
			if err := ipam.ReleasePool(pID); err != nil {
				log.G(context.TODO()).Warnf("Failed to release overlapping pool %s while returning from pool request helper for network %s", pool, nwName)
			}
		}
	}()

	for {
		poolID, pool, meta, err = ipam.RequestPool(addressSpace, requestedPool, requestedSubPool, options, v6)
		if err != nil {
			return "", nil, nil, err
		}

		// If the network pool was explicitly chosen, the network belongs to
		// global scope, or it is invalid ("0.0.0.0/0"), then we don't perform
		// check for overlaps.
		//
		// FIXME(thaJeztah): why are we ignoring invalid pools here?
		//
		// The "invalid" conditions was added in [libnetwork#1095][1], which
		// moved code to reduce os-specific dependencies in the ipam package,
		// but also introduced a types.IsIPNetValid() function, which considers
		// "0.0.0.0/0" invalid, and added it to the conditions below.
		//
		// Unfortunately review does not mention this change, so there's no
		// context why. Possibly this was done to prevent errors further down
		// the line (when checking for overlaps), but returning an error here
		// instead would likely have avoided that as well, so we can only guess.
		//
		// [1]: https://github.com/moby/libnetwork/commit/5ca79d6b87873264516323a7b76f0af7d0298492#diff-bdcd879439d041827d334846f9aba01de6e3683ed8fdd01e63917dae6df23846
		if requestedPool != "" || n.Scope() == scope.Global || pool.String() == "0.0.0.0/0" {
			return poolID, pool, meta, nil
		}

		// Check for overlap and if none found, we have found the right pool.
		if _, err := netutils.FindAvailableNetwork([]*net.IPNet{pool}); err == nil {
			return poolID, pool, meta, nil
		}

		// Pool obtained in this iteration is overlapping. Hold onto the pool
		// and don't release it yet, because we don't want IPAM to give us back
		// the same pool over again. But make sure we still do a deferred release
		// when we have either obtained a non-overlapping pool or ran out of
		// pre-defined pools.
		tmpPoolLeases = append(tmpPoolLeases, poolID)
	}
}

func (n *Network) ipamAllocateVersion(ipVer int, ipam ipamapi.Ipam) error {
	var (
		cfgList  *[]*IpamConf
		infoList *[]*IpamInfo
		err      error
	)

	switch ipVer {
	case 4:
		cfgList = &n.ipamV4Config
		infoList = &n.ipamV4Info
	case 6:
		cfgList = &n.ipamV6Config
		infoList = &n.ipamV6Info
	default:
		return types.InternalErrorf("incorrect ip version passed to ipam allocate: %d", ipVer)
	}

	if len(*cfgList) == 0 {
		*cfgList = []*IpamConf{{}}
	}

	*infoList = make([]*IpamInfo, len(*cfgList))

	log.G(context.TODO()).Debugf("Allocating IPv%d pools for network %s (%s)", ipVer, n.Name(), n.ID())

	for i, cfg := range *cfgList {
		if err = cfg.Validate(); err != nil {
			return err
		}
		d := &IpamInfo{}
		(*infoList)[i] = d

		d.AddressSpace = n.addrSpace
		d.PoolID, d.Pool, d.Meta, err = n.requestPoolHelper(ipam, n.addrSpace, cfg.PreferredPool, cfg.SubPool, n.ipamOptions, ipVer == 6)
		if err != nil {
			return err
		}

		defer func() {
			if err != nil {
				if err := ipam.ReleasePool(d.PoolID); err != nil {
					log.G(context.TODO()).Warnf("Failed to release address pool %s after failure to create network %s (%s)", d.PoolID, n.Name(), n.ID())
				}
			}
		}()

		if gws, ok := d.Meta[netlabel.Gateway]; ok {
			if d.Gateway, err = types.ParseCIDR(gws); err != nil {
				return types.BadRequestErrorf("failed to parse gateway address (%v) returned by ipam driver: %v", gws, err)
			}
		}

		// If user requested a specific gateway, libnetwork will allocate it
		// irrespective of whether ipam driver returned a gateway already.
		// If none of the above is true, libnetwork will allocate one.
		if cfg.Gateway != "" || d.Gateway == nil {
			gatewayOpts := map[string]string{
				ipamapi.RequestAddressType: netlabel.Gateway,
			}
			if d.Gateway, _, err = ipam.RequestAddress(d.PoolID, net.ParseIP(cfg.Gateway), gatewayOpts); err != nil {
				return types.InternalErrorf("failed to allocate gateway (%v): %v", cfg.Gateway, err)
			}
		}

		// Auxiliary addresses must be part of the master address pool
		// If they fall into the container addressable pool, libnetwork will reserve them
		if cfg.AuxAddresses != nil {
			var ip net.IP
			d.IPAMData.AuxAddresses = make(map[string]*net.IPNet, len(cfg.AuxAddresses))
			for k, v := range cfg.AuxAddresses {
				if ip = net.ParseIP(v); ip == nil {
					return types.BadRequestErrorf("non parsable secondary ip address (%s:%s) passed for network %s", k, v, n.Name())
				}
				if !d.Pool.Contains(ip) {
					return types.ForbiddenErrorf("auxiliary address: (%s:%s) must belong to the master pool: %s", k, v, d.Pool)
				}
				// Attempt reservation in the container addressable pool, silent the error if address does not belong to that pool
				if d.IPAMData.AuxAddresses[k], _, err = ipam.RequestAddress(d.PoolID, ip, nil); err != nil && err != ipamapi.ErrIPOutOfRange {
					return types.InternalErrorf("failed to allocate secondary ip address (%s:%s): %v", k, v, err)
				}
			}
		}
	}

	return nil
}

func (n *Network) ipamRelease() {
	if n.hasSpecialDriver() {
		return
	}
	ipam, _, err := n.getController().getIPAMDriver(n.ipamType)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to retrieve ipam driver to release address pool(s) on delete of network %s (%s): %v", n.Name(), n.ID(), err)
		return
	}
	n.ipamReleaseVersion(4, ipam)
	n.ipamReleaseVersion(6, ipam)
}

func (n *Network) ipamReleaseVersion(ipVer int, ipam ipamapi.Ipam) {
	var infoList *[]*IpamInfo

	switch ipVer {
	case 4:
		infoList = &n.ipamV4Info
	case 6:
		infoList = &n.ipamV6Info
	default:
		log.G(context.TODO()).Warnf("incorrect ip version passed to ipam release: %d", ipVer)
		return
	}

	if len(*infoList) == 0 {
		return
	}

	log.G(context.TODO()).Debugf("releasing IPv%d pools from network %s (%s)", ipVer, n.Name(), n.ID())

	for _, d := range *infoList {
		if d.Gateway != nil {
			if err := ipam.ReleaseAddress(d.PoolID, d.Gateway.IP); err != nil {
				log.G(context.TODO()).Warnf("Failed to release gateway ip address %s on delete of network %s (%s): %v", d.Gateway.IP, n.Name(), n.ID(), err)
			}
		}
		if d.IPAMData.AuxAddresses != nil {
			for k, nw := range d.IPAMData.AuxAddresses {
				if d.Pool.Contains(nw.IP) {
					if err := ipam.ReleaseAddress(d.PoolID, nw.IP); err != nil && err != ipamapi.ErrIPOutOfRange {
						log.G(context.TODO()).Warnf("Failed to release secondary ip address %s (%v) on delete of network %s (%s): %v", k, nw.IP, n.Name(), n.ID(), err)
					}
				}
			}
		}
		if err := ipam.ReleasePool(d.PoolID); err != nil {
			log.G(context.TODO()).Warnf("Failed to release address pool %s on delete of network %s (%s): %v", d.PoolID, n.Name(), n.ID(), err)
		}
	}

	*infoList = nil
}

func (n *Network) getIPInfo(ipVer int) []*IpamInfo {
	var info []*IpamInfo
	switch ipVer {
	case 4:
		info = n.ipamV4Info
	case 6:
		info = n.ipamV6Info
	default:
		return nil
	}
	l := make([]*IpamInfo, 0, len(info))
	n.mu.Lock()
	l = append(l, info...)
	n.mu.Unlock()
	return l
}

func (n *Network) getIPData(ipVer int) []driverapi.IPAMData {
	var info []*IpamInfo
	switch ipVer {
	case 4:
		info = n.ipamV4Info
	case 6:
		info = n.ipamV6Info
	default:
		return nil
	}
	l := make([]driverapi.IPAMData, 0, len(info))
	n.mu.Lock()
	for _, d := range info {
		l = append(l, d.IPAMData)
	}
	n.mu.Unlock()
	return l
}

func (n *Network) deriveAddressSpace() (string, error) {
	ipam, _ := n.getController().ipamRegistry.IPAM(n.ipamType)
	if ipam == nil {
		return "", types.NotFoundErrorf("failed to get default address space: unknown ipam type %q", n.ipamType)
	}
	local, global, err := ipam.GetDefaultAddressSpaces()
	if err != nil {
		return "", types.NotFoundErrorf("failed to get default address space: %v", err)
	}
	if n.DataScope() == scope.Global {
		return global, nil
	}
	return local, nil
}

// Peers returns a slice of PeerInfo structures which has the information about the peer
// nodes participating in the same overlay network. This is currently the per-network
// gossip cluster. For non-dynamic overlay networks and bridge networks it returns an
// empty slice
func (n *Network) Peers() []networkdb.PeerInfo {
	if !n.Dynamic() {
		return []networkdb.PeerInfo{}
	}

	a := n.getController().getAgent()
	if a == nil {
		return []networkdb.PeerInfo{}
	}

	return a.networkDB.Peers(n.ID())
}

func (n *Network) DriverOptions() map[string]string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.generic != nil {
		if m, ok := n.generic[netlabel.GenericData]; ok {
			return m.(map[string]string)
		}
	}
	return map[string]string{}
}

func (n *Network) Scope() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.scope
}

func (n *Network) IpamConfig() (ipamType string, ipamOptions map[string]string, ipamV4Config []*IpamConf, ipamV6Config []*IpamConf) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ipamV4Config = make([]*IpamConf, len(n.ipamV4Config))
	for i, c := range n.ipamV4Config {
		cc := &IpamConf{}
		if err := c.CopyTo(cc); err != nil {
			log.G(context.TODO()).WithError(err).Error("Error copying ipam ipv4 config")
		}
		ipamV4Config[i] = cc
	}

	ipamV6Config = make([]*IpamConf, len(n.ipamV6Config))
	for i, c := range n.ipamV6Config {
		cc := &IpamConf{}
		if err := c.CopyTo(cc); err != nil {
			log.G(context.TODO()).WithError(err).Debug("Error copying ipam ipv6 config")
		}
		ipamV6Config[i] = cc
	}

	return n.ipamType, n.ipamOptions, ipamV4Config, ipamV6Config
}

func (n *Network) IpamInfo() (ipamV4Info []*IpamInfo, ipamV6Info []*IpamInfo) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ipamV4Info = make([]*IpamInfo, len(n.ipamV4Info))
	for i, info := range n.ipamV4Info {
		ic := &IpamInfo{}
		if err := info.CopyTo(ic); err != nil {
			log.G(context.TODO()).WithError(err).Error("Error copying IPv4 IPAM config")
		}
		ipamV4Info[i] = ic
	}

	ipamV6Info = make([]*IpamInfo, len(n.ipamV6Info))
	for i, info := range n.ipamV6Info {
		ic := &IpamInfo{}
		if err := info.CopyTo(ic); err != nil {
			log.G(context.TODO()).WithError(err).Error("Error copying IPv6 IPAM config")
		}
		ipamV6Info[i] = ic
	}

	return ipamV4Info, ipamV6Info
}

func (n *Network) Internal() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.internal
}

func (n *Network) Attachable() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.attachable
}

func (n *Network) Ingress() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.ingress
}

func (n *Network) Dynamic() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.dynamic
}

func (n *Network) IPv6Enabled() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.enableIPv6
}

func (n *Network) ConfigFrom() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.configFrom
}

func (n *Network) ConfigOnly() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.configOnly
}

func (n *Network) Labels() map[string]string {
	n.mu.Lock()
	defer n.mu.Unlock()

	lbls := make(map[string]string, len(n.labels))
	for k, v := range n.labels {
		lbls[k] = v
	}

	return lbls
}

func (n *Network) TableEventRegister(tableName string, objType driverapi.ObjectType) error {
	if !driverapi.IsValidType(objType) {
		return fmt.Errorf("invalid object type %v in registering table, %s", objType, tableName)
	}

	t := networkDBTable{
		name:    tableName,
		objType: objType,
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.driverTables = append(n.driverTables, t)
	return nil
}

func (n *Network) UpdateIpamConfig(ipV4Data []driverapi.IPAMData) {
	ipamV4Config := make([]*IpamConf, len(ipV4Data))

	for i, data := range ipV4Data {
		ic := &IpamConf{}
		ic.PreferredPool = data.Pool.String()
		ic.Gateway = data.Gateway.IP.String()
		ipamV4Config[i] = ic
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.ipamV4Config = ipamV4Config
}

// Special drivers are ones which do not need to perform any Network plumbing
func (n *Network) hasSpecialDriver() bool {
	return n.Type() == "host" || n.Type() == "null"
}

func (n *Network) hasLoadBalancerEndpoint() bool {
	return len(n.loadBalancerIP) != 0
}

func (n *Network) ResolveName(req string, ipType int) ([]net.IP, bool) {
	var ipv6Miss bool

	c := n.getController()
	networkID := n.ID()
	c.mu.Lock()
	defer c.mu.Unlock()
	sr, ok := c.svcRecords[networkID]

	if !ok {
		return nil, false
	}

	req = strings.TrimSuffix(req, ".")
	req = strings.ToLower(req)
	ipSet, ok := sr.svcMap.Get(req)

	if ipType == types.IPv6 {
		// If the name resolved to v4 address then its a valid name in
		// the docker network domain. If the network is not v6 enabled
		// set ipv6Miss to filter the DNS query from going to external
		// resolvers.
		if ok && !n.enableIPv6 {
			ipv6Miss = true
		}
		ipSet, ok = sr.svcIPv6Map.Get(req)
	}

	if ok && len(ipSet) > 0 {
		// this map is to avoid IP duplicates, this can happen during a transition period where 2 services are using the same IP
		noDup := make(map[string]bool)
		var ipLocal []net.IP
		for _, ip := range ipSet {
			if _, dup := noDup[ip.ip]; !dup {
				noDup[ip.ip] = true
				ipLocal = append(ipLocal, net.ParseIP(ip.ip))
			}
		}
		return ipLocal, ok
	}

	return nil, ipv6Miss
}

func (n *Network) HandleQueryResp(name string, ip net.IP) {
	networkID := n.ID()
	c := n.getController()
	c.mu.Lock()
	defer c.mu.Unlock()
	sr, ok := c.svcRecords[networkID]

	if !ok {
		return
	}

	ipStr := netutils.ReverseIP(ip.String())
	// If an object with extResolver == true is already in the set this call will fail
	// but anyway it means that has already been inserted before
	if ok, _ := sr.ipMap.Contains(ipStr, ipInfo{name: name}); ok {
		sr.ipMap.Remove(ipStr, ipInfo{name: name})
		sr.ipMap.Insert(ipStr, ipInfo{name: name, extResolver: true})
	}
}

func (n *Network) ResolveIP(ip string) string {
	networkID := n.ID()
	c := n.getController()
	c.mu.Lock()
	defer c.mu.Unlock()
	sr, ok := c.svcRecords[networkID]

	if !ok {
		return ""
	}

	nwName := n.Name()

	elemSet, ok := sr.ipMap.Get(ip)
	if !ok || len(elemSet) == 0 {
		return ""
	}
	// NOTE it is possible to have more than one element in the Set, this will happen
	// because of interleave of different events from different sources (local container create vs
	// network db notifications)
	// In such cases the resolution will be based on the first element of the set, and can vary
	// during the system stabilitation
	elem := elemSet[0]
	if elem.extResolver {
		return ""
	}

	return elem.name + "." + nwName
}

func (n *Network) ResolveService(name string) ([]*net.SRV, []net.IP) {
	c := n.getController()

	srv := []*net.SRV{}
	ip := []net.IP{}

	log.G(context.TODO()).Debugf("Service name To resolve: %v", name)

	// There are DNS implementations that allow SRV queries for names not in
	// the format defined by RFC 2782. Hence specific validations checks are
	// not done
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		return nil, nil
	}

	portName := parts[0]
	proto := parts[1]
	svcName := strings.Join(parts[2:], ".")

	networkID := n.ID()
	c.mu.Lock()
	defer c.mu.Unlock()
	sr, ok := c.svcRecords[networkID]

	if !ok {
		return nil, nil
	}

	svcs, ok := sr.service[svcName]
	if !ok {
		return nil, nil
	}

	for _, svc := range svcs {
		if svc.portName != portName {
			continue
		}
		if svc.proto != proto {
			continue
		}
		for _, t := range svc.target {
			srv = append(srv,
				&net.SRV{
					Target: t.name,
					Port:   t.port,
				})

			ip = append(ip, t.ip)
		}
	}

	return srv, ip
}

func (n *Network) ExecFunc(f func()) error {
	return types.NotImplementedErrorf("ExecFunc not supported by network")
}

func (n *Network) NdotsSet() bool {
	return false
}

// config-only network is looked up by name
func (c *Controller) getConfigNetwork(name string) (*Network, error) {
	var n *Network
	c.WalkNetworks(func(current *Network) bool {
		if current.ConfigOnly() && current.Name() == name {
			n = current
			return true
		}
		return false
	})

	if n == nil {
		return nil, types.NotFoundErrorf("configuration network %q not found", name)
	}

	return n, nil
}

func (n *Network) lbSandboxName() string {
	name := "lb-" + n.name
	if n.ingress {
		name = n.name + "-sbox"
	}
	return name
}

func (n *Network) lbEndpointName() string {
	return n.name + "-endpoint"
}

func (n *Network) createLoadBalancerSandbox() (retErr error) {
	sandboxName := n.lbSandboxName()
	// Mark the sandbox to be a load balancer
	sbOptions := []SandboxOption{OptionLoadBalancer(n.id)}
	if n.ingress {
		sbOptions = append(sbOptions, OptionIngress())
	}
	sb, err := n.ctrlr.NewSandbox(sandboxName, sbOptions...)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			if e := n.ctrlr.SandboxDestroy(sandboxName); e != nil {
				log.G(context.TODO()).Warnf("could not delete sandbox %s on failure on failure (%v): %v", sandboxName, retErr, e)
			}
		}
	}()

	endpointName := n.lbEndpointName()
	epOptions := []EndpointOption{
		CreateOptionIpam(n.loadBalancerIP, nil, nil, nil),
		CreateOptionLoadBalancer(),
	}
	if n.hasLoadBalancerEndpoint() && !n.ingress {
		// Mark LB endpoints as anonymous so they don't show up in DNS
		epOptions = append(epOptions, CreateOptionAnonymous())
	}
	ep, err := n.createEndpoint(endpointName, epOptions...)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			if e := ep.Delete(true); e != nil {
				log.G(context.TODO()).Warnf("could not delete endpoint %s on failure on failure (%v): %v", endpointName, retErr, e)
			}
		}
	}()

	if err := ep.Join(sb, nil); err != nil {
		return err
	}

	return sb.EnableService()
}

func (n *Network) deleteLoadBalancerSandbox() error {
	n.mu.Lock()
	c := n.ctrlr
	name := n.name
	n.mu.Unlock()

	sandboxName := n.lbSandboxName()
	endpointName := n.lbEndpointName()

	endpoint, err := n.EndpointByName(endpointName)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to find load balancer endpoint %s on network %s: %v", endpointName, name, err)
	} else {
		info := endpoint.Info()
		if info != nil {
			sb := info.Sandbox()
			if sb != nil {
				if err := sb.DisableService(); err != nil {
					log.G(context.TODO()).Warnf("Failed to disable service on sandbox %s: %v", sandboxName, err)
					// Ignore error and attempt to delete the load balancer endpoint
				}
			}
		}

		if err := endpoint.Delete(true); err != nil {
			log.G(context.TODO()).Warnf("Failed to delete endpoint %s (%s) in %s: %v", endpoint.Name(), endpoint.ID(), sandboxName, err)
			// Ignore error and attempt to delete the sandbox.
		}
	}

	if err := c.SandboxDestroy(sandboxName); err != nil {
		return fmt.Errorf("Failed to delete %s sandbox: %v", sandboxName, err)
	}
	return nil
}
