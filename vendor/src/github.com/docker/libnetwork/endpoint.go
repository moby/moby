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
	"github.com/docker/libnetwork/netlabel"
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
	epMap["generic"] = ep.generic
	epMap["sandbox"] = ep.sandboxID
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

	if epMap["generic"] != nil {
		ep.generic = epMap["generic"].(map[string]interface{})
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
	return ep.getNetwork().name
}

// endpoint Key structure : endpoint/network-id/endpoint-id
func (ep *endpoint) Key() []string {
	return []string{datastore.EndpointKeyPrefix, ep.getNetwork().id, ep.id}
}

func (ep *endpoint) KeyPrefix() []string {
	return []string{datastore.EndpointKeyPrefix, ep.getNetwork().id}
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

	ep.Lock()
	if ep.sandboxID != "" {
		ep.Unlock()
		return types.ForbiddenErrorf("a sandbox has already joined the endpoint")
	}

	ep.sandboxID = sbox.ID()
	ep.joinInfo = &endpointJoinInfo{}
	network := ep.network
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
	driver := network.driver
	nid := network.id
	network.Unlock()

	ep.processOptions(options...)

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
	if err = sb.updateHostsFile(address, network.getSvcRecords()); err != nil {
		return err
	}

	if err = sb.updateDNS(ep.getNetwork().enableIPv6); err != nil {
		return err
	}

	if !ep.isLocalScoped() {
		if err = network.ctrlr.updateToStore(ep); err != nil {
			return err
		}
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

	ep.Lock()
	ep.sandboxID = ""
	n := ep.network
	ep.Unlock()

	n.Lock()
	c := n.ctrlr
	d := n.driver
	n.Unlock()

	if !ep.isLocalScoped() {
		if err := c.updateToStore(ep); err != nil {
			ep.Lock()
			ep.sandboxID = sid
			ep.Unlock()
			return err
		}
	}

	if err := d.Leave(n.id, ep.id); err != nil {
		return err
	}

	if err := sb.clearNetworkResources(ep); err != nil {
		return err
	}

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
	ep.Lock()
	epid := ep.id
	name := ep.name
	n := ep.network
	if ep.sandboxID != "" {
		ep.Unlock()
		return &ActiveContainerError{name: name, id: epid}
	}
	n.Lock()
	ctrlr := n.ctrlr
	n.Unlock()
	ep.Unlock()

	if !ep.isLocalScoped() {
		if err = ctrlr.deleteFromStore(ep); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			ep.dbExists = false
			if !ep.isLocalScoped() {
				if e := ctrlr.updateToStore(ep); e != nil {
					log.Warnf("failed to recreate endpoint in store %s : %v", name, e)
				}
			}
		}
	}()

	// Update the endpoint count in network and update it in the datastore
	n.DecEndpointCnt()
	if err = ctrlr.updateToStore(n); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			n.IncEndpointCnt()
			if e := ctrlr.updateToStore(n); e != nil {
				log.Warnf("failed to update network %s : %v", n.name, e)
			}
		}
	}()

	if err = ep.deleteEndpoint(); err != nil {
		return err
	}

	return nil
}

func (ep *endpoint) deleteEndpoint() error {
	ep.Lock()
	n := ep.network
	name := ep.name
	epid := ep.id
	ep.Unlock()

	n.Lock()
	_, ok := n.endpoints[epid]
	if !ok {
		n.Unlock()
		return nil
	}

	nid := n.id
	driver := n.driver
	delete(n.endpoints, epid)
	n.Unlock()

	if err := driver.DeleteEndpoint(nid, epid); err != nil {
		if _, ok := err.(types.ForbiddenError); ok {
			n.Lock()
			n.endpoints[epid] = ep
			n.Unlock()
			return err
		}
		log.Warnf("driver error deleting endpoint %s : %v", name, err)
	}

	n.updateSvcRecord(ep, false)
	return nil
}

func (ep *endpoint) getNetwork() *network {
	ep.Lock()
	defer ep.Unlock()
	return ep.network
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

	if ep.iface != nil {
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

func (ep *endpoint) DataScope() datastore.DataScope {
	ep.Lock()
	defer ep.Unlock()
	return ep.network.dataScope
}

func (ep *endpoint) isLocalScoped() bool {
	return ep.DataScope() == datastore.LocalScope
}
