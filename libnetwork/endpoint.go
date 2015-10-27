package libnetwork

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
)

// Endpoint represents a logical connection between a network and a sandbox.
type Endpoint interface {
	// A system generated id for this endpoint.
	ID() string

	// Name returns the name of this endpoint.
	Name() string

	// Network returns the name of the network to which this endpoint is attached.
	Network() string

	// Join joins the sandbox to the endpoint and populates into the sandbox
	// the network resources allocated for the endpoint.
	Join(sandbox Sandbox, options ...EndpointOption) error

	// Leave detaches the network resources populated in the sandbox.
	Leave(sandbox Sandbox, options ...EndpointOption) error

	// Return certain operational data belonging to this endpoint
	Info() EndpointInfo

	// DriverInfo returns a collection of driver operational data related to this endpoint retrieved from the driver
	DriverInfo() (map[string]interface{}, error)

	// Delete and detaches this endpoint from the network.
	Delete() error
}

// EndpointOption is a option setter function type used to pass varios options to Network
// and Endpoint interfaces methods. The various setter functions of type EndpointOption are
// provided by libnetwork, they look like <Create|Join|Leave>Option[...](...)
type EndpointOption func(ep *endpoint)

type endpoint struct {
	name          string
	id            string
	network       *network
	iface         *endpointInterface
	joinInfo      *endpointJoinInfo
	sandboxID     string
	exposedPorts  []types.TransportPort
	anonymous     bool
	generic       map[string]interface{}
	joinLeaveDone chan struct{}
	dbIndex       uint64
	dbExists      bool
	sync.Mutex
}

func (ep *endpoint) MarshalJSON() ([]byte, error) {
	ep.Lock()
	defer ep.Unlock()

	epMap := make(map[string]interface{})
	epMap["name"] = ep.name
	epMap["id"] = ep.id
	epMap["ep_iface"] = ep.iface
	epMap["exposed_ports"] = ep.exposedPorts
	if ep.generic != nil {
		epMap["generic"] = ep.generic
	}
	epMap["sandbox"] = ep.sandboxID
	epMap["anonymous"] = ep.anonymous
	return json.Marshal(epMap)
}

func (ep *endpoint) UnmarshalJSON(b []byte) (err error) {
	ep.Lock()
	defer ep.Unlock()

	var epMap map[string]interface{}
	if err := json.Unmarshal(b, &epMap); err != nil {
		return err
	}
	ep.name = epMap["name"].(string)
	ep.id = epMap["id"].(string)

	ib, _ := json.Marshal(epMap["ep_iface"])
	json.Unmarshal(ib, &ep.iface)

	tb, _ := json.Marshal(epMap["exposed_ports"])
	var tPorts []types.TransportPort
	json.Unmarshal(tb, &tPorts)
	ep.exposedPorts = tPorts

	cb, _ := json.Marshal(epMap["sandbox"])
	json.Unmarshal(cb, &ep.sandboxID)

	if v, ok := epMap["generic"]; ok {
		ep.generic = v.(map[string]interface{})

		if opt, ok := ep.generic[netlabel.PortMap]; ok {
			pblist := []types.PortBinding{}

			for i := 0; i < len(opt.([]interface{})); i++ {
				pb := types.PortBinding{}
				tmp := opt.([]interface{})[i].(map[string]interface{})

				bytes, err := json.Marshal(tmp)
				if err != nil {
					log.Error(err)
					break
				}
				err = json.Unmarshal(bytes, &pb)
				if err != nil {
					log.Error(err)
					break
				}
				pblist = append(pblist, pb)
			}
			ep.generic[netlabel.PortMap] = pblist
		}

		if opt, ok := ep.generic[netlabel.ExposedPorts]; ok {
			tplist := []types.TransportPort{}

			for i := 0; i < len(opt.([]interface{})); i++ {
				tp := types.TransportPort{}
				tmp := opt.([]interface{})[i].(map[string]interface{})

				bytes, err := json.Marshal(tmp)
				if err != nil {
					log.Error(err)
					break
				}
				err = json.Unmarshal(bytes, &tp)
				if err != nil {
					log.Error(err)
					break
				}
				tplist = append(tplist, tp)
			}
			ep.generic[netlabel.ExposedPorts] = tplist

		}
	}

	if v, ok := epMap["anonymous"]; ok {
		ep.anonymous = v.(bool)
	}
	return nil
}

func (ep *endpoint) New() datastore.KVObject {
	return &endpoint{network: ep.getNetwork()}
}

func (ep *endpoint) CopyTo(o datastore.KVObject) error {
	ep.Lock()
	defer ep.Unlock()

	dstEp := o.(*endpoint)
	dstEp.name = ep.name
	dstEp.id = ep.id
	dstEp.sandboxID = ep.sandboxID
	dstEp.dbIndex = ep.dbIndex
	dstEp.dbExists = ep.dbExists
	dstEp.anonymous = ep.anonymous

	if ep.iface != nil {
		dstEp.iface = &endpointInterface{}
		ep.iface.CopyTo(dstEp.iface)
	}

	dstEp.exposedPorts = make([]types.TransportPort, len(ep.exposedPorts))
	copy(dstEp.exposedPorts, ep.exposedPorts)

	dstEp.generic = options.Generic{}
	for k, v := range ep.generic {
		dstEp.generic[k] = v
	}

	return nil
}

func (ep *endpoint) ID() string {
	ep.Lock()
	defer ep.Unlock()

	return ep.id
}

func (ep *endpoint) Name() string {
	ep.Lock()
	defer ep.Unlock()

	return ep.name
}

func (ep *endpoint) Network() string {
	if ep.network == nil {
		return ""
	}

	return ep.network.name
}

func (ep *endpoint) isAnonymous() bool {
	ep.Lock()
	defer ep.Unlock()
	return ep.anonymous
}

// endpoint Key structure : endpoint/network-id/endpoint-id
func (ep *endpoint) Key() []string {
	if ep.network == nil {
		return nil
	}

	return []string{datastore.EndpointKeyPrefix, ep.network.id, ep.id}
}

func (ep *endpoint) KeyPrefix() []string {
	if ep.network == nil {
		return nil
	}

	return []string{datastore.EndpointKeyPrefix, ep.network.id}
}

func (ep *endpoint) networkIDFromKey(key string) (string, error) {
	// endpoint Key structure : docker/libnetwork/endpoint/${network-id}/${endpoint-id}
	// it's an invalid key if the key doesn't have all the 5 key elements above
	keyElements := strings.Split(key, "/")
	if !strings.HasPrefix(key, datastore.Key(datastore.EndpointKeyPrefix)) || len(keyElements) < 5 {
		return "", fmt.Errorf("invalid endpoint key : %v", key)
	}
	// network-id is placed at index=3. pls refer to endpoint.Key() method
	return strings.Split(key, "/")[3], nil
}

func (ep *endpoint) Value() []byte {
	b, err := json.Marshal(ep)
	if err != nil {
		return nil
	}
	return b
}

func (ep *endpoint) SetValue(value []byte) error {
	return json.Unmarshal(value, ep)
}

func (ep *endpoint) Index() uint64 {
	ep.Lock()
	defer ep.Unlock()
	return ep.dbIndex
}

func (ep *endpoint) SetIndex(index uint64) {
	ep.Lock()
	defer ep.Unlock()
	ep.dbIndex = index
	ep.dbExists = true
}

func (ep *endpoint) Exists() bool {
	ep.Lock()
	defer ep.Unlock()
	return ep.dbExists
}

func (ep *endpoint) Skip() bool {
	return ep.getNetwork().Skip()
}

func (ep *endpoint) processOptions(options ...EndpointOption) {
	ep.Lock()
	defer ep.Unlock()

	for _, opt := range options {
		if opt != nil {
			opt(ep)
		}
	}
}

func (ep *endpoint) getNetwork() *network {
	ep.Lock()
	defer ep.Unlock()

	return ep.network
}

func (ep *endpoint) getNetworkFromStore() (*network, error) {
	if ep.network == nil {
		return nil, fmt.Errorf("invalid network object in endpoint %s", ep.Name())
	}

	return ep.network.ctrlr.getNetworkFromStore(ep.network.id)
}

func (ep *endpoint) Join(sbox Sandbox, options ...EndpointOption) error {
	if sbox == nil {
		return types.BadRequestErrorf("endpoint cannot be joined by nil container")
	}

	sb, ok := sbox.(*sandbox)
	if !ok {
		return types.BadRequestErrorf("not a valid Sandbox interface")
	}

	sb.joinLeaveStart()
	defer sb.joinLeaveEnd()

	return ep.sbJoin(sbox, options...)
}

func (ep *endpoint) sbJoin(sbox Sandbox, options ...EndpointOption) error {
	var err error
	sb, ok := sbox.(*sandbox)
	if !ok {
		return types.BadRequestErrorf("not a valid Sandbox interface")
	}

	network, err := ep.getNetworkFromStore()
	if err != nil {
		return fmt.Errorf("failed to get network from store during join: %v", err)
	}

	ep, err = network.getEndpointFromStore(ep.ID())
	if err != nil {
		return fmt.Errorf("failed to get endpoint from store during join: %v", err)
	}

	ep.Lock()
	if ep.sandboxID != "" {
		ep.Unlock()
		return types.ForbiddenErrorf("another container is attached to the same network endpoint")
	}
	ep.Unlock()

	ep.Lock()
	ep.network = network
	ep.sandboxID = sbox.ID()
	ep.joinInfo = &endpointJoinInfo{}
	epid := ep.id
	ep.Unlock()
	defer func() {
		if err != nil {
			ep.Lock()
			ep.sandboxID = ""
			ep.Unlock()
		}
	}()

	network.Lock()
	nid := network.id
	network.Unlock()

	ep.processOptions(options...)

	driver, err := network.driver()
	if err != nil {
		return fmt.Errorf("failed to join endpoint: %v", err)
	}

	err = driver.Join(nid, epid, sbox.Key(), ep, sbox.Labels())
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			// Do not alter global err variable, it's needed by the previous defer
			if err := driver.Leave(nid, epid); err != nil {
				log.Warnf("driver leave failed while rolling back join: %v", err)
			}
		}
	}()

	address := ""
	if ip := ep.getFirstInterfaceAddress(); ip != nil {
		address = ip.String()
	}
	if err = sb.updateHostsFile(address, network.getSvcRecords(ep)); err != nil {
		return err
	}

	// Watch for service records
	network.getController().watchSvcRecord(ep)

	if err = sb.updateDNS(network.enableIPv6); err != nil {
		return err
	}

	if err = network.getController().updateToStore(ep); err != nil {
		return err
	}

	sb.Lock()
	heap.Push(&sb.endpoints, ep)
	sb.Unlock()
	defer func() {
		if err != nil {
			for i, e := range sb.getConnectedEndpoints() {
				if e == ep {
					sb.Lock()
					heap.Remove(&sb.endpoints, i)
					sb.Unlock()
					return
				}
			}
		}
	}()

	if err = sb.populateNetworkResources(ep); err != nil {
		return err
	}

	if sb.needDefaultGW() {
		return sb.setupDefaultGW(ep)
	}
	return sb.clearDefaultGW()
}

func (ep *endpoint) rename(name string) error {
	var err error
	n := ep.getNetwork()
	if n == nil {
		return fmt.Errorf("network not connected for ep %q", ep.name)
	}

	n.getController().Lock()
	netWatch, ok := n.getController().nmap[n.ID()]
	n.getController().Unlock()

	if !ok {
		return fmt.Errorf("watch null for network %q", n.Name())
	}

	n.updateSvcRecord(ep, n.getController().getLocalEps(netWatch), false)

	oldName := ep.name
	ep.name = name

	n.updateSvcRecord(ep, n.getController().getLocalEps(netWatch), true)
	defer func() {
		if err != nil {
			n.updateSvcRecord(ep, n.getController().getLocalEps(netWatch), false)
			ep.name = oldName
			n.updateSvcRecord(ep, n.getController().getLocalEps(netWatch), true)
		}
	}()

	// Update the store with the updated name
	if err = n.getController().updateToStore(ep); err != nil {
		return err
	}
	// After the name change do a dummy endpoint count update to
	// trigger the service record update in the peer nodes

	// Ignore the error because updateStore fail for EpCnt is a
	// benign error. Besides there is no meaningful recovery that
	// we can do. When the cluster recovers subsequent EpCnt update
	// will force the peers to get the correct EP name.
	n.getEpCnt().updateStore()

	return err
}

func (ep *endpoint) hasInterface(iName string) bool {
	ep.Lock()
	defer ep.Unlock()

	return ep.iface != nil && ep.iface.srcName == iName
}

func (ep *endpoint) Leave(sbox Sandbox, options ...EndpointOption) error {
	if sbox == nil || sbox.ID() == "" || sbox.Key() == "" {
		return types.BadRequestErrorf("invalid Sandbox passed to enpoint leave: %v", sbox)
	}

	sb, ok := sbox.(*sandbox)
	if !ok {
		return types.BadRequestErrorf("not a valid Sandbox interface")
	}

	sb.joinLeaveStart()
	defer sb.joinLeaveEnd()

	return ep.sbLeave(sbox, options...)
}

func (ep *endpoint) sbLeave(sbox Sandbox, options ...EndpointOption) error {
	sb, ok := sbox.(*sandbox)
	if !ok {
		return types.BadRequestErrorf("not a valid Sandbox interface")
	}

	n, err := ep.getNetworkFromStore()
	if err != nil {
		return fmt.Errorf("failed to get network from store during leave: %v", err)
	}

	ep, err = n.getEndpointFromStore(ep.ID())
	if err != nil {
		return fmt.Errorf("failed to get endpoint from store during leave: %v", err)
	}

	ep.Lock()
	sid := ep.sandboxID
	ep.Unlock()

	if sid == "" {
		return types.ForbiddenErrorf("cannot leave endpoint with no attached sandbox")
	}
	if sid != sbox.ID() {
		return types.ForbiddenErrorf("unexpected sandbox ID in leave request. Expected %s. Got %s", ep.sandboxID, sbox.ID())
	}

	ep.processOptions(options...)

	d, err := n.driver()
	if err != nil {
		return fmt.Errorf("failed to leave endpoint: %v", err)
	}

	ep.Lock()
	ep.sandboxID = ""
	ep.network = n
	ep.Unlock()

	if err := d.Leave(n.id, ep.id); err != nil {
		if _, ok := err.(types.MaskableError); !ok {
			log.Warnf("driver error disconnecting container %s : %v", ep.name, err)
		}
	}

	if err := sb.clearNetworkResources(ep); err != nil {
		log.Warnf("Could not cleanup network resources on container %s disconnect: %v", ep.name, err)
	}

	// Update the store about the sandbox detach only after we
	// have completed sb.clearNetworkresources above to avoid
	// spurious logs when cleaning up the sandbox when the daemon
	// ungracefully exits and restarts before completing sandbox
	// detach but after store has been updated.
	if err := n.getController().updateToStore(ep); err != nil {
		return err
	}

	sb.deleteHostsEntries(n.getSvcRecords(ep))

	if sb.needDefaultGW() {
		ep := sb.getEPwithoutGateway()
		if ep == nil {
			return fmt.Errorf("endpoint without GW expected, but not found")
		}
		return sb.setupDefaultGW(ep)
	}
	return sb.clearDefaultGW()
}

func (ep *endpoint) Delete() error {
	var err error
	n, err := ep.getNetworkFromStore()
	if err != nil {
		return fmt.Errorf("failed to get network during Delete: %v", err)
	}

	ep, err = n.getEndpointFromStore(ep.ID())
	if err != nil {
		return fmt.Errorf("failed to get endpoint from store during Delete: %v", err)
	}

	ep.Lock()
	epid := ep.id
	name := ep.name
	if ep.sandboxID != "" {
		ep.Unlock()
		return &ActiveContainerError{name: name, id: epid}
	}
	ep.Unlock()

	if err = n.getController().deleteFromStore(ep); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			ep.dbExists = false
			if e := n.getController().updateToStore(ep); e != nil {
				log.Warnf("failed to recreate endpoint in store %s : %v", name, e)
			}
		}
	}()

	if err = n.getEpCnt().DecEndpointCnt(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if e := n.getEpCnt().IncEndpointCnt(); e != nil {
				log.Warnf("failed to update network %s : %v", n.name, e)
			}
		}
	}()

	// unwatch for service records
	n.getController().unWatchSvcRecord(ep)

	if err = ep.deleteEndpoint(); err != nil {
		return err
	}

	ep.releaseAddress()

	return nil
}

func (ep *endpoint) deleteEndpoint() error {
	ep.Lock()
	n := ep.network
	name := ep.name
	epid := ep.id
	ep.Unlock()

	driver, err := n.driver()
	if err != nil {
		return fmt.Errorf("failed to delete endpoint: %v", err)
	}

	if err := driver.DeleteEndpoint(n.id, epid); err != nil {
		if _, ok := err.(types.ForbiddenError); ok {
			return err
		}

		if _, ok := err.(types.MaskableError); !ok {
			log.Warnf("driver error deleting endpoint %s : %v", name, err)
		}
	}

	return nil
}

func (ep *endpoint) getSandbox() (*sandbox, bool) {
	ep.Lock()
	c := ep.network.getController()
	sid := ep.sandboxID
	ep.Unlock()

	c.Lock()
	ps, ok := c.sandboxes[sid]
	c.Unlock()

	return ps, ok
}

func (ep *endpoint) getFirstInterfaceAddress() net.IP {
	ep.Lock()
	defer ep.Unlock()

	if ep.iface.addr != nil {
		return ep.iface.addr.IP
	}

	return nil
}

// EndpointOptionGeneric function returns an option setter for a Generic option defined
// in a Dictionary of Key-Value pair
func EndpointOptionGeneric(generic map[string]interface{}) EndpointOption {
	return func(ep *endpoint) {
		for k, v := range generic {
			ep.generic[k] = v
		}
	}
}

// CreateOptionExposedPorts function returns an option setter for the container exposed
// ports option to be passed to network.CreateEndpoint() method.
func CreateOptionExposedPorts(exposedPorts []types.TransportPort) EndpointOption {
	return func(ep *endpoint) {
		// Defensive copy
		eps := make([]types.TransportPort, len(exposedPorts))
		copy(eps, exposedPorts)
		// Store endpoint label and in generic because driver needs it
		ep.exposedPorts = eps
		ep.generic[netlabel.ExposedPorts] = eps
	}
}

// CreateOptionPortMapping function returns an option setter for the mapping
// ports option to be passed to network.CreateEndpoint() method.
func CreateOptionPortMapping(portBindings []types.PortBinding) EndpointOption {
	return func(ep *endpoint) {
		// Store a copy of the bindings as generic data to pass to the driver
		pbs := make([]types.PortBinding, len(portBindings))
		copy(pbs, portBindings)
		ep.generic[netlabel.PortMap] = pbs
	}
}

// CreateOptionAnonymous function returns an option setter for setting
// this endpoint as anonymous
func CreateOptionAnonymous() EndpointOption {
	return func(ep *endpoint) {
		ep.anonymous = true
	}
}

// JoinOptionPriority function returns an option setter for priority option to
// be passed to the endpoint.Join() method.
func JoinOptionPriority(ep Endpoint, prio int) EndpointOption {
	return func(ep *endpoint) {
		// ep lock already acquired
		c := ep.network.getController()
		c.Lock()
		sb, ok := c.sandboxes[ep.sandboxID]
		c.Unlock()
		if !ok {
			log.Errorf("Could not set endpoint priority value during Join to endpoint %s: No sandbox id present in endpoint", ep.id)
			return
		}
		sb.epPriority[ep.id] = prio
	}
}

func (ep *endpoint) DataScope() string {
	return ep.getNetwork().DataScope()
}

func (ep *endpoint) assignAddress() error {
	var (
		ipam ipamapi.Ipam
		err  error
	)

	n := ep.getNetwork()
	if n.Type() == "host" || n.Type() == "null" {
		return nil
	}

	log.Debugf("Assigning addresses for endpoint %s's interface on network %s", ep.Name(), n.Name())

	ipam, err = n.getController().getIpamDriver(n.ipamType)
	if err != nil {
		return err
	}
	err = ep.assignAddressVersion(4, ipam)
	if err != nil {
		return err
	}
	return ep.assignAddressVersion(6, ipam)
}

func (ep *endpoint) assignAddressVersion(ipVer int, ipam ipamapi.Ipam) error {
	var (
		poolID  *string
		address **net.IPNet
	)

	n := ep.getNetwork()
	switch ipVer {
	case 4:
		poolID = &ep.iface.v4PoolID
		address = &ep.iface.addr
	case 6:
		poolID = &ep.iface.v6PoolID
		address = &ep.iface.addrv6
	default:
		return types.InternalErrorf("incorrect ip version number passed: %d", ipVer)
	}

	ipInfo := n.getIPInfo(ipVer)

	// ipv6 address is not mandatory
	if len(ipInfo) == 0 && ipVer == 6 {
		return nil
	}

	for _, d := range ipInfo {
		addr, _, err := ipam.RequestAddress(d.PoolID, nil, nil)
		if err == nil {
			ep.Lock()
			*address = addr
			*poolID = d.PoolID
			ep.Unlock()
			return nil
		}
		if err != ipamapi.ErrNoAvailableIPs {
			return err
		}
	}
	return fmt.Errorf("no available IPv%d addresses on this network's address pools: %s (%s)", ipVer, n.Name(), n.ID())
}

func (ep *endpoint) releaseAddress() {
	n := ep.getNetwork()
	if n.Type() == "host" || n.Type() == "null" {
		return
	}

	log.Debugf("Releasing addresses for endpoint %s's interface on network %s", ep.Name(), n.Name())

	ipam, err := n.getController().getIpamDriver(n.ipamType)
	if err != nil {
		log.Warnf("Failed to retrieve ipam driver to release interface address on delete of endpoint %s (%s): %v", ep.Name(), ep.ID(), err)
		return
	}
	if err := ipam.ReleaseAddress(ep.iface.v4PoolID, ep.iface.addr.IP); err != nil {
		log.Warnf("Failed to release ip address %s on delete of endpoint %s (%s): %v", ep.iface.addr.IP, ep.Name(), ep.ID(), err)
	}
	if ep.iface.addrv6 != nil && ep.iface.addrv6.IP.IsGlobalUnicast() {
		if err := ipam.ReleaseAddress(ep.iface.v6PoolID, ep.iface.addrv6.IP); err != nil {
			log.Warnf("Failed to release ip address %s on delete of endpoint %s (%s): %v", ep.iface.addrv6.IP, ep.Name(), ep.ID(), err)
		}
	}
}

func (c *controller) cleanupLocalEndpoints() {
	nl, err := c.getNetworksForScope(datastore.LocalScope)
	if err != nil {
		log.Warnf("Could not get list of networks during endpoint cleanup: %v", err)
		return
	}

	for _, n := range nl {
		epl, err := n.getEndpointsFromStore()
		if err != nil {
			log.Warnf("Could not get list of endpoints in network %s during endpoint cleanup: %v", n.name, err)
			continue
		}

		for _, ep := range epl {
			if err := ep.Delete(); err != nil {
				log.Warnf("Could not delete local endpoint %s during endpoint cleanup: %v", ep.name, err)
			}
		}
	}
}
