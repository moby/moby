// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.19

package libnetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/sliceutil"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
)

// ByNetworkType sorts a [Endpoint] slice based on the network-type
// they're attached to. It implements [sort.Interface] and can be used
// with [sort.Stable] or [sort.Sort]. It is used by [Sandbox.ResolveName]
// when resolving names in swarm mode. In swarm mode, services with exposed
// ports are connected to user overlay network, ingress network, and local
// ("docker_gwbridge") networks. Name resolution should prioritize returning
// the VIP/IPs on user overlay network over ingress and local networks.
//
// ByNetworkType re-orders the endpoints based on the network-type they
// are attached to:
//
//  1. dynamic networks (user overlay networks)
//  2. ingress network(s)
//  3. local networks ("docker_gwbridge")
type ByNetworkType []*Endpoint

func (ep ByNetworkType) Len() int      { return len(ep) }
func (ep ByNetworkType) Swap(i, j int) { ep[i], ep[j] = ep[j], ep[i] }
func (ep ByNetworkType) Less(i, j int) bool {
	return getNetworkType(ep[i].getNetwork()) < getNetworkType(ep[j].getNetwork())
}

// Define the order in which resolution should happen if an endpoint is
// attached to multiple network-types. It is used by [ByNetworkType].
const (
	typeDynamic = iota
	typeIngress
	typeLocal
)

func getNetworkType(nw *Network) int {
	switch {
	case nw.ingress:
		return typeIngress
	case nw.dynamic:
		return typeDynamic
	default:
		return typeLocal
	}
}

// EndpointOption is an option setter function type used to pass various options to Network
// and Endpoint interfaces methods. The various setter functions of type EndpointOption are
// provided by libnetwork, they look like <Create|Join|Leave>Option[...](...)
type EndpointOption func(ep *Endpoint)

// Endpoint represents a logical connection between a network and a sandbox.
type Endpoint struct {
	name         string
	id           string
	network      *Network
	iface        *EndpointInterface
	joinInfo     *endpointJoinInfo
	sandboxID    string
	exposedPorts []types.TransportPort
	// dnsNames holds all the non-fully qualified DNS names associated to this endpoint. Order matters: first entry
	// will be used for the PTR records associated to the endpoint's IPv4 and IPv6 addresses.
	dnsNames          []string
	disableResolution bool
	generic           map[string]interface{}
	prefAddress       net.IP
	prefAddressV6     net.IP
	ipamOptions       map[string]string
	aliases           map[string]string
	svcID             string
	svcName           string
	virtualIP         net.IP
	svcAliases        []string
	ingressPorts      []*PortConfig
	dbIndex           uint64
	dbExists          bool
	serviceEnabled    bool
	loadBalancer      bool
	mu                sync.Mutex
}

func (ep *Endpoint) MarshalJSON() ([]byte, error) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	epMap := make(map[string]interface{})
	epMap["name"] = ep.name
	epMap["id"] = ep.id
	epMap["ep_iface"] = ep.iface
	epMap["joinInfo"] = ep.joinInfo
	epMap["exposed_ports"] = ep.exposedPorts
	if ep.generic != nil {
		epMap["generic"] = ep.generic
	}
	epMap["sandbox"] = ep.sandboxID
	epMap["dnsNames"] = ep.dnsNames
	epMap["disableResolution"] = ep.disableResolution
	epMap["svcName"] = ep.svcName
	epMap["svcID"] = ep.svcID
	epMap["virtualIP"] = ep.virtualIP.String()
	epMap["ingressPorts"] = ep.ingressPorts
	epMap["svcAliases"] = ep.svcAliases
	epMap["loadBalancer"] = ep.loadBalancer

	return json.Marshal(epMap)
}

func (ep *Endpoint) UnmarshalJSON(b []byte) (err error) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	var epMap map[string]interface{}
	if err := json.Unmarshal(b, &epMap); err != nil {
		return err
	}
	ep.name = epMap["name"].(string)
	ep.id = epMap["id"].(string)

	// TODO(cpuguy83): So yeah, this isn't checking any errors anywhere.
	// Seems like we should be checking errors even because of memory related issues that can arise.
	// Alas it seems like given the nature of this data we could introduce problems if we start checking these errors.
	//
	// If anyone ever comes here and figures out one way or another if we can/should be checking these errors and it turns out we can't... then please document *why*

	ib, _ := json.Marshal(epMap["ep_iface"])
	json.Unmarshal(ib, &ep.iface) //nolint:errcheck

	jb, _ := json.Marshal(epMap["joinInfo"])
	json.Unmarshal(jb, &ep.joinInfo) //nolint:errcheck

	tb, _ := json.Marshal(epMap["exposed_ports"])
	var tPorts []types.TransportPort
	json.Unmarshal(tb, &tPorts) //nolint:errcheck
	ep.exposedPorts = tPorts

	cb, _ := json.Marshal(epMap["sandbox"])
	json.Unmarshal(cb, &ep.sandboxID) //nolint:errcheck

	if v, ok := epMap["generic"]; ok {
		ep.generic = v.(map[string]interface{})

		if opt, ok := ep.generic[netlabel.PortMap]; ok {
			pblist := []types.PortBinding{}

			for i := 0; i < len(opt.([]interface{})); i++ {
				pb := types.PortBinding{}
				tmp := opt.([]interface{})[i].(map[string]interface{})

				bytes, err := json.Marshal(tmp)
				if err != nil {
					log.G(context.TODO()).Error(err)
					break
				}
				err = json.Unmarshal(bytes, &pb)
				if err != nil {
					log.G(context.TODO()).Error(err)
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
					log.G(context.TODO()).Error(err)
					break
				}
				err = json.Unmarshal(bytes, &tp)
				if err != nil {
					log.G(context.TODO()).Error(err)
					break
				}
				tplist = append(tplist, tp)
			}
			ep.generic[netlabel.ExposedPorts] = tplist
		}
	}

	var anonymous bool
	if v, ok := epMap["anonymous"]; ok {
		anonymous = v.(bool)
	}
	if v, ok := epMap["disableResolution"]; ok {
		ep.disableResolution = v.(bool)
	}

	if sn, ok := epMap["svcName"]; ok {
		ep.svcName = sn.(string)
	}

	if si, ok := epMap["svcID"]; ok {
		ep.svcID = si.(string)
	}

	if vip, ok := epMap["virtualIP"]; ok {
		ep.virtualIP = net.ParseIP(vip.(string))
	}

	if v, ok := epMap["loadBalancer"]; ok {
		ep.loadBalancer = v.(bool)
	}

	sal, _ := json.Marshal(epMap["svcAliases"])
	var svcAliases []string
	json.Unmarshal(sal, &svcAliases) //nolint:errcheck
	ep.svcAliases = svcAliases

	pc, _ := json.Marshal(epMap["ingressPorts"])
	var ingressPorts []*PortConfig
	json.Unmarshal(pc, &ingressPorts) //nolint:errcheck
	ep.ingressPorts = ingressPorts

	ma, _ := json.Marshal(epMap["myAliases"])
	var myAliases []string
	json.Unmarshal(ma, &myAliases) //nolint:errcheck

	_, hasDNSNames := epMap["dnsNames"]
	dn, _ := json.Marshal(epMap["dnsNames"])
	var dnsNames []string
	json.Unmarshal(dn, &dnsNames)
	ep.dnsNames = dnsNames

	// TODO(aker): remove this migration code in v27
	if !hasDNSNames {
		// The field dnsNames was introduced in v25.0. If we don't have it, the on-disk state was written by an older
		// daemon, thus we need to populate dnsNames based off of myAliases and anonymous values.
		if !anonymous {
			myAliases = append([]string{ep.name}, myAliases...)
		}
		ep.dnsNames = sliceutil.Dedup(myAliases)
	}

	return nil
}

func (ep *Endpoint) New() datastore.KVObject {
	return &Endpoint{network: ep.getNetwork()}
}

func (ep *Endpoint) CopyTo(o datastore.KVObject) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	dstEp := o.(*Endpoint)
	dstEp.name = ep.name
	dstEp.id = ep.id
	dstEp.sandboxID = ep.sandboxID
	dstEp.dbIndex = ep.dbIndex
	dstEp.dbExists = ep.dbExists
	dstEp.disableResolution = ep.disableResolution
	dstEp.svcName = ep.svcName
	dstEp.svcID = ep.svcID
	dstEp.virtualIP = ep.virtualIP
	dstEp.loadBalancer = ep.loadBalancer

	dstEp.svcAliases = make([]string, len(ep.svcAliases))
	copy(dstEp.svcAliases, ep.svcAliases)

	dstEp.ingressPorts = make([]*PortConfig, len(ep.ingressPorts))
	copy(dstEp.ingressPorts, ep.ingressPorts)

	if ep.iface != nil {
		dstEp.iface = &EndpointInterface{}
		if err := ep.iface.CopyTo(dstEp.iface); err != nil {
			return err
		}
	}

	if ep.joinInfo != nil {
		dstEp.joinInfo = &endpointJoinInfo{}
		if err := ep.joinInfo.CopyTo(dstEp.joinInfo); err != nil {
			return err
		}
	}

	dstEp.exposedPorts = make([]types.TransportPort, len(ep.exposedPorts))
	copy(dstEp.exposedPorts, ep.exposedPorts)

	dstEp.dnsNames = make([]string, len(ep.dnsNames))
	copy(dstEp.dnsNames, ep.dnsNames)

	dstEp.generic = options.Generic{}
	for k, v := range ep.generic {
		dstEp.generic[k] = v
	}

	return nil
}

// ID returns the system-generated id for this endpoint.
func (ep *Endpoint) ID() string {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	return ep.id
}

// Name returns the name of this endpoint.
func (ep *Endpoint) Name() string {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	return ep.name
}

// Network returns the name of the network to which this endpoint is attached.
func (ep *Endpoint) Network() string {
	if ep.network == nil {
		return ""
	}

	return ep.network.name
}

// getDNSNames returns a copy of the DNS names associated to this endpoint. The first entry is the one used for PTR
// records.
func (ep *Endpoint) getDNSNames() []string {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	dnsNames := make([]string, len(ep.dnsNames))
	copy(dnsNames, ep.dnsNames)
	return dnsNames
}

// isServiceEnabled check if service is enabled on the endpoint
func (ep *Endpoint) isServiceEnabled() bool {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.serviceEnabled
}

// enableService sets service enabled on the endpoint
func (ep *Endpoint) enableService() {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.serviceEnabled = true
}

// disableService disables service on the endpoint
func (ep *Endpoint) disableService() {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.serviceEnabled = false
}

func (ep *Endpoint) needResolver() bool {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return !ep.disableResolution
}

// endpoint Key structure : endpoint/network-id/endpoint-id
func (ep *Endpoint) Key() []string {
	if ep.network == nil {
		return nil
	}

	return []string{datastore.EndpointKeyPrefix, ep.network.id, ep.id}
}

func (ep *Endpoint) KeyPrefix() []string {
	if ep.network == nil {
		return nil
	}

	return []string{datastore.EndpointKeyPrefix, ep.network.id}
}

func (ep *Endpoint) Value() []byte {
	b, err := json.Marshal(ep)
	if err != nil {
		return nil
	}
	return b
}

func (ep *Endpoint) SetValue(value []byte) error {
	return json.Unmarshal(value, ep)
}

func (ep *Endpoint) Index() uint64 {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.dbIndex
}

func (ep *Endpoint) SetIndex(index uint64) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.dbIndex = index
	ep.dbExists = true
}

func (ep *Endpoint) Exists() bool {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.dbExists
}

func (ep *Endpoint) Skip() bool {
	return ep.getNetwork().Skip()
}

func (ep *Endpoint) processOptions(options ...EndpointOption) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	for _, opt := range options {
		if opt != nil {
			opt(ep)
		}
	}
}

func (ep *Endpoint) getNetwork() *Network {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	return ep.network
}

func (ep *Endpoint) getNetworkFromStore() (*Network, error) {
	if ep.network == nil {
		return nil, fmt.Errorf("invalid network object in endpoint %s", ep.Name())
	}

	return ep.network.getController().getNetworkFromStore(ep.network.id)
}

// Join joins the sandbox to the endpoint and populates into the sandbox
// the network resources allocated for the endpoint.
func (ep *Endpoint) Join(sb *Sandbox, options ...EndpointOption) error {
	if sb == nil || sb.ID() == "" || sb.Key() == "" {
		return types.InvalidParameterErrorf("invalid Sandbox passed to endpoint join: %v", sb)
	}

	sb.joinLeaveStart()
	defer sb.joinLeaveEnd()

	return ep.sbJoin(sb, options...)
}

func (ep *Endpoint) sbJoin(sb *Sandbox, options ...EndpointOption) (err error) {
	n, err := ep.getNetworkFromStore()
	if err != nil {
		return fmt.Errorf("failed to get network from store during join: %v", err)
	}

	ep, err = n.getEndpointFromStore(ep.ID())
	if err != nil {
		return fmt.Errorf("failed to get endpoint from store during join: %v", err)
	}

	ep.mu.Lock()
	if ep.sandboxID != "" {
		ep.mu.Unlock()
		return types.ForbiddenErrorf("another container is attached to the same network endpoint")
	}
	ep.network = n
	ep.sandboxID = sb.ID()
	ep.joinInfo = &endpointJoinInfo{}
	epid := ep.id
	ep.mu.Unlock()
	defer func() {
		if err != nil {
			ep.mu.Lock()
			ep.sandboxID = ""
			ep.mu.Unlock()
		}
	}()

	nid := n.ID()

	ep.processOptions(options...)

	d, err := n.driver(true)
	if err != nil {
		return fmt.Errorf("failed to get driver during join: %v", err)
	}

	err = d.Join(nid, epid, sb.Key(), ep, sb.Labels())
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if e := d.Leave(nid, epid); e != nil {
				log.G(context.TODO()).Warnf("driver leave failed while rolling back join: %v", e)
			}
		}
	}()

	if !n.getController().isAgent() {
		if !n.getController().isSwarmNode() || n.Scope() != scope.Swarm || !n.driverIsMultihost() {
			n.updateSvcRecord(ep, true)
		}
	}

	if err := sb.updateHostsFile(ep.getEtcHostsAddrs()); err != nil {
		return err
	}
	if err = sb.updateDNS(n.enableIPv6); err != nil {
		return err
	}

	// Current endpoint providing external connectivity for the sandbox
	extEp := sb.getGatewayEndpoint()

	sb.addEndpoint(ep)
	defer func() {
		if err != nil {
			sb.removeEndpoint(ep)
		}
	}()

	if err = sb.populateNetworkResources(ep); err != nil {
		return err
	}

	if err = n.getController().updateToStore(ep); err != nil {
		return err
	}

	if err = ep.addDriverInfoToCluster(); err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if e := ep.deleteDriverInfoFromCluster(); e != nil {
				log.G(context.TODO()).Errorf("Could not delete endpoint state for endpoint %s from cluster on join failure: %v", ep.Name(), e)
			}
		}
	}()

	// Load balancing endpoints should never have a default gateway nor
	// should they alter the status of a network's default gateway
	if ep.loadBalancer && !sb.ingress {
		return nil
	}

	if sb.needDefaultGW() && sb.getEndpointInGWNetwork() == nil {
		return sb.setupDefaultGW()
	}

	// Enable upstream forwarding if the sandbox gained external connectivity.
	if sb.resolver != nil {
		sb.resolver.SetForwardingPolicy(sb.hasExternalAccess())
	}

	currentExtEp := sb.getGatewayEndpoint()
	moveExtConn := currentExtEp != extEp
	if moveExtConn {
		if extEp != nil {
			log.G(context.TODO()).Debugf("Revoking external connectivity on endpoint %s (%s)", extEp.Name(), extEp.ID())
			extN, err := extEp.getNetworkFromStore()
			if err != nil {
				return fmt.Errorf("failed to get network from store for revoking external connectivity during join: %v", err)
			}
			extD, err := extN.driver(true)
			if err != nil {
				return fmt.Errorf("failed to get driver for revoking external connectivity during join: %v", err)
			}
			if err = extD.RevokeExternalConnectivity(extEp.network.ID(), extEp.ID()); err != nil {
				return types.InternalErrorf(
					"driver failed revoking external connectivity on endpoint %s (%s): %v",
					extEp.Name(), extEp.ID(), err)
			}
			defer func() {
				if err != nil {
					if e := extD.ProgramExternalConnectivity(extEp.network.ID(), extEp.ID(), sb.Labels()); e != nil {
						log.G(context.TODO()).Warnf("Failed to roll-back external connectivity on endpoint %s (%s): %v",
							extEp.Name(), extEp.ID(), e)
					}
				}
			}()
		}
		if !n.internal {
			log.G(context.TODO()).Debugf("Programming external connectivity on endpoint %s (%s)", ep.Name(), ep.ID())
			if err = d.ProgramExternalConnectivity(n.ID(), ep.ID(), sb.Labels()); err != nil {
				return types.InternalErrorf(
					"driver failed programming external connectivity on endpoint %s (%s): %v",
					ep.Name(), ep.ID(), err)
			}
		}
	}

	if !sb.needDefaultGW() {
		if e := sb.clearDefaultGW(); e != nil {
			log.G(context.TODO()).Warnf("Failure while disconnecting sandbox %s (%s) from gateway network: %v",
				sb.ID(), sb.ContainerID(), e)
		}
	}

	return nil
}

func (ep *Endpoint) rename(name string) error {
	ep.mu.Lock()
	ep.name = name
	ep.mu.Unlock()

	// Update the store with the updated name
	if err := ep.getNetwork().getController().updateToStore(ep); err != nil {
		return err
	}

	return nil
}

func (ep *Endpoint) UpdateDNSNames(dnsNames []string) error {
	nw := ep.getNetwork()
	c := nw.getController()
	sb, ok := ep.getSandbox()
	if !ok {
		log.G(context.TODO()).WithFields(log.Fields{
			"sandboxID":  ep.sandboxID,
			"endpointID": ep.ID(),
		}).Warn("DNSNames update aborted, sandbox is not present anymore")
		return nil
	}

	if c.isAgent() {
		if err := ep.deleteServiceInfoFromCluster(sb, true, "UpdateDNSNames"); err != nil {
			return types.InternalErrorf("could not delete service state for endpoint %s from cluster on UpdateDNSNames: %v", ep.Name(), err)
		}

		ep.dnsNames = dnsNames
		if err := ep.addServiceInfoToCluster(sb); err != nil {
			return types.InternalErrorf("could not add service state for endpoint %s to cluster on UpdateDNSNames: %v", ep.Name(), err)
		}
	} else {
		nw.updateSvcRecord(ep, false)

		ep.dnsNames = dnsNames
		nw.updateSvcRecord(ep, true)
	}

	// Update the store with the updated name
	if err := c.updateToStore(ep); err != nil {
		return err
	}

	return nil
}

func (ep *Endpoint) hasInterface(iName string) bool {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	return ep.iface != nil && ep.iface.srcName == iName
}

// Leave detaches the network resources populated in the sandbox.
func (ep *Endpoint) Leave(sb *Sandbox) error {
	if sb == nil || sb.ID() == "" || sb.Key() == "" {
		return types.InvalidParameterErrorf("invalid Sandbox passed to endpoint leave: %v", sb)
	}

	sb.joinLeaveStart()
	defer sb.joinLeaveEnd()

	return ep.sbLeave(sb, false)
}

func (ep *Endpoint) sbLeave(sb *Sandbox, force bool) error {
	n, err := ep.getNetworkFromStore()
	if err != nil {
		return fmt.Errorf("failed to get network from store during leave: %v", err)
	}

	ep, err = n.getEndpointFromStore(ep.ID())
	if err != nil {
		return fmt.Errorf("failed to get endpoint from store during leave: %v", err)
	}

	ep.mu.Lock()
	sid := ep.sandboxID
	ep.mu.Unlock()

	if sid == "" {
		return types.ForbiddenErrorf("cannot leave endpoint with no attached sandbox")
	}
	if sid != sb.ID() {
		return types.ForbiddenErrorf("unexpected sandbox ID in leave request. Expected %s. Got %s", ep.sandboxID, sb.ID())
	}

	d, err := n.driver(!force)
	if err != nil {
		return fmt.Errorf("failed to get driver during endpoint leave: %v", err)
	}

	ep.mu.Lock()
	ep.sandboxID = ""
	ep.network = n
	ep.mu.Unlock()

	// Current endpoint providing external connectivity to the sandbox
	extEp := sb.getGatewayEndpoint()
	moveExtConn := extEp != nil && (extEp.ID() == ep.ID())

	if d != nil {
		if moveExtConn {
			log.G(context.TODO()).Debugf("Revoking external connectivity on endpoint %s (%s)", ep.Name(), ep.ID())
			if err := d.RevokeExternalConnectivity(n.id, ep.id); err != nil {
				log.G(context.TODO()).Warnf("driver failed revoking external connectivity on endpoint %s (%s): %v",
					ep.Name(), ep.ID(), err)
			}
		}

		if err := d.Leave(n.id, ep.id); err != nil {
			if _, ok := err.(types.MaskableError); !ok {
				log.G(context.TODO()).Warnf("driver error disconnecting container %s : %v", ep.name, err)
			}
		}
	}

	if err := ep.deleteServiceInfoFromCluster(sb, true, "sbLeave"); err != nil {
		log.G(context.TODO()).Warnf("Failed to clean up service info on container %s disconnect: %v", ep.name, err)
	}

	if err := sb.clearNetworkResources(ep); err != nil {
		log.G(context.TODO()).Warnf("Failed to clean up network resources on container %s disconnect: %v", ep.name, err)
	}

	// Update the store about the sandbox detach only after we
	// have completed sb.clearNetworkresources above to avoid
	// spurious logs when cleaning up the sandbox when the daemon
	// ungracefully exits and restarts before completing sandbox
	// detach but after store has been updated.
	if err := n.getController().updateToStore(ep); err != nil {
		return err
	}

	if e := ep.deleteDriverInfoFromCluster(); e != nil {
		log.G(context.TODO()).Errorf("Failed to delete endpoint state for endpoint %s from cluster: %v", ep.Name(), e)
	}

	sb.deleteHostsEntries(n.getSvcRecords(ep))
	if !sb.inDelete && sb.needDefaultGW() && sb.getEndpointInGWNetwork() == nil {
		return sb.setupDefaultGW()
	}

	// Disable upstream forwarding if the sandbox lost external connectivity.
	if sb.resolver != nil {
		sb.resolver.SetForwardingPolicy(sb.hasExternalAccess())
	}

	// New endpoint providing external connectivity for the sandbox
	extEp = sb.getGatewayEndpoint()
	if moveExtConn && extEp != nil {
		log.G(context.TODO()).Debugf("Programming external connectivity on endpoint %s (%s)", extEp.Name(), extEp.ID())
		extN, err := extEp.getNetworkFromStore()
		if err != nil {
			return fmt.Errorf("failed to get network from store for programming external connectivity during leave: %v", err)
		}
		extD, err := extN.driver(true)
		if err != nil {
			return fmt.Errorf("failed to get driver for programming external connectivity during leave: %v", err)
		}
		if err := extD.ProgramExternalConnectivity(extEp.network.ID(), extEp.ID(), sb.Labels()); err != nil {
			log.G(context.TODO()).Warnf("driver failed programming external connectivity on endpoint %s: (%s) %v",
				extEp.Name(), extEp.ID(), err)
		}
	}

	if !sb.needDefaultGW() {
		if err := sb.clearDefaultGW(); err != nil {
			log.G(context.TODO()).Warnf("Failure while disconnecting sandbox %s (%s) from gateway network: %v",
				sb.ID(), sb.ContainerID(), err)
		}
	}

	return nil
}

// Delete deletes and detaches this endpoint from the network.
func (ep *Endpoint) Delete(force bool) error {
	var err error
	n, err := ep.getNetworkFromStore()
	if err != nil {
		return fmt.Errorf("failed to get network during Delete: %v", err)
	}

	ep, err = n.getEndpointFromStore(ep.ID())
	if err != nil {
		return fmt.Errorf("failed to get endpoint from store during Delete: %v", err)
	}

	ep.mu.Lock()
	epid := ep.id
	name := ep.name
	sbid := ep.sandboxID
	ep.mu.Unlock()

	sb, _ := n.getController().SandboxByID(sbid)
	if sb != nil && !force {
		return &ActiveContainerError{name: name, id: epid}
	}

	if sb != nil {
		if e := ep.sbLeave(sb, force); e != nil {
			log.G(context.TODO()).Warnf("failed to leave sandbox for endpoint %s : %v", name, e)
		}
	}

	if err = n.getController().deleteFromStore(ep); err != nil {
		return err
	}

	defer func() {
		if err != nil && !force {
			ep.dbExists = false
			if e := n.getController().updateToStore(ep); e != nil {
				log.G(context.TODO()).Warnf("failed to recreate endpoint in store %s : %v", name, e)
			}
		}
	}()

	if !n.getController().isSwarmNode() || n.Scope() != scope.Swarm || !n.driverIsMultihost() {
		n.updateSvcRecord(ep, false)
	}

	if err = ep.deleteEndpoint(force); err != nil && !force {
		return err
	}

	ep.releaseAddress()

	if err := n.getEpCnt().DecEndpointCnt(); err != nil {
		log.G(context.TODO()).Warnf("failed to decrement endpoint count for ep %s: %v", ep.ID(), err)
	}

	return nil
}

func (ep *Endpoint) deleteEndpoint(force bool) error {
	ep.mu.Lock()
	n := ep.network
	name := ep.name
	epid := ep.id
	ep.mu.Unlock()

	driver, err := n.driver(!force)
	if err != nil {
		return fmt.Errorf("failed to delete endpoint: %v", err)
	}

	if driver == nil {
		return nil
	}

	if err := driver.DeleteEndpoint(n.id, epid); err != nil {
		if _, ok := err.(types.ForbiddenError); ok {
			return err
		}

		if _, ok := err.(types.MaskableError); !ok {
			log.G(context.TODO()).Warnf("driver error deleting endpoint %s : %v", name, err)
		}
	}

	return nil
}

func (ep *Endpoint) getSandbox() (*Sandbox, bool) {
	c := ep.network.getController()
	ep.mu.Lock()
	sid := ep.sandboxID
	ep.mu.Unlock()

	c.mu.Lock()
	ps, ok := c.sandboxes[sid]
	c.mu.Unlock()

	return ps, ok
}

// Return a list of this endpoint's addresses to add to '/etc/hosts'.
func (ep *Endpoint) getEtcHostsAddrs() []string {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	// Do not update hosts file with internal network's endpoint IP
	if n := ep.network; n == nil || n.ingress || n.Name() == libnGWNetwork {
		return nil
	}

	var addresses []string
	if ep.iface.addr != nil {
		addresses = append(addresses, ep.iface.addr.IP.String())
	}
	if ep.iface.addrv6 != nil {
		addresses = append(addresses, ep.iface.addrv6.IP.String())
	}
	return addresses
}

// EndpointOptionGeneric function returns an option setter for a Generic option defined
// in a Dictionary of Key-Value pair
func EndpointOptionGeneric(generic map[string]interface{}) EndpointOption {
	return func(ep *Endpoint) {
		for k, v := range generic {
			ep.generic[k] = v
		}
	}
}

var (
	linkLocalMask     = net.CIDRMask(16, 32)
	linkLocalMaskIPv6 = net.CIDRMask(64, 128)
)

// CreateOptionIpam function returns an option setter for the ipam configuration for this endpoint
func CreateOptionIpam(ipV4, ipV6 net.IP, llIPs []net.IP, ipamOptions map[string]string) EndpointOption {
	return func(ep *Endpoint) {
		ep.prefAddress = ipV4
		ep.prefAddressV6 = ipV6
		if len(llIPs) != 0 {
			for _, ip := range llIPs {
				nw := &net.IPNet{IP: ip, Mask: linkLocalMask}
				if ip.To4() == nil {
					nw.Mask = linkLocalMaskIPv6
				}
				ep.iface.llAddrs = append(ep.iface.llAddrs, nw)
			}
		}
		ep.ipamOptions = ipamOptions
	}
}

// CreateOptionExposedPorts function returns an option setter for the container exposed
// ports option to be passed to [Network.CreateEndpoint] method.
func CreateOptionExposedPorts(exposedPorts []types.TransportPort) EndpointOption {
	return func(ep *Endpoint) {
		// Defensive copy
		eps := make([]types.TransportPort, len(exposedPorts))
		copy(eps, exposedPorts)
		// Store endpoint label and in generic because driver needs it
		ep.exposedPorts = eps
		ep.generic[netlabel.ExposedPorts] = eps
	}
}

// CreateOptionPortMapping function returns an option setter for the mapping
// ports option to be passed to [Network.CreateEndpoint] method.
func CreateOptionPortMapping(portBindings []types.PortBinding) EndpointOption {
	return func(ep *Endpoint) {
		// Store a copy of the bindings as generic data to pass to the driver
		pbs := make([]types.PortBinding, len(portBindings))
		copy(pbs, portBindings)
		ep.generic[netlabel.PortMap] = pbs
	}
}

// CreateOptionDNS function returns an option setter for dns entry option to
// be passed to container Create method.
func CreateOptionDNS(dns []string) EndpointOption {
	return func(ep *Endpoint) {
		ep.generic[netlabel.DNSServers] = dns
	}
}

// CreateOptionDNSNames specifies the list of (non fully qualified) DNS names associated to an endpoint. These will be
// used to populate the embedded DNS server. Order matters: first name will be used to generate PTR records.
func CreateOptionDNSNames(names []string) EndpointOption {
	return func(ep *Endpoint) {
		ep.dnsNames = names
	}
}

// CreateOptionDisableResolution function returns an option setter to indicate
// this endpoint doesn't want embedded DNS server functionality
func CreateOptionDisableResolution() EndpointOption {
	return func(ep *Endpoint) {
		ep.disableResolution = true
	}
}

// CreateOptionAlias function returns an option setter for setting endpoint alias
func CreateOptionAlias(name string, alias string) EndpointOption {
	return func(ep *Endpoint) {
		if ep.aliases == nil {
			ep.aliases = make(map[string]string)
		}
		ep.aliases[alias] = name
	}
}

// CreateOptionService function returns an option setter for setting service binding configuration
func CreateOptionService(name, id string, vip net.IP, ingressPorts []*PortConfig, aliases []string) EndpointOption {
	return func(ep *Endpoint) {
		ep.svcName = name
		ep.svcID = id
		ep.virtualIP = vip
		ep.ingressPorts = ingressPorts
		ep.svcAliases = aliases
	}
}

// CreateOptionLoadBalancer function returns an option setter for denoting the endpoint is a load balancer for a network
func CreateOptionLoadBalancer() EndpointOption {
	return func(ep *Endpoint) {
		ep.loadBalancer = true
	}
}

// JoinOptionPriority function returns an option setter for priority option to
// be passed to the endpoint.Join() method.
func JoinOptionPriority(prio int) EndpointOption {
	return func(ep *Endpoint) {
		// ep lock already acquired
		c := ep.network.getController()
		c.mu.Lock()
		sb, ok := c.sandboxes[ep.sandboxID]
		c.mu.Unlock()
		if !ok {
			log.G(context.TODO()).Errorf("Could not set endpoint priority value during Join to endpoint %s: No sandbox id present in endpoint", ep.id)
			return
		}
		sb.epPriority[ep.id] = prio
	}
}

func (ep *Endpoint) assignAddress(ipam ipamapi.Ipam, assignIPv4, assignIPv6 bool) error {
	var err error

	n := ep.getNetwork()
	if n.hasSpecialDriver() {
		return nil
	}

	log.G(context.TODO()).Debugf("Assigning addresses for endpoint %s's interface on network %s", ep.Name(), n.Name())

	if assignIPv4 {
		if err = ep.assignAddressVersion(4, ipam); err != nil {
			return err
		}
	}

	if assignIPv6 {
		err = ep.assignAddressVersion(6, ipam)
	}

	return err
}

func (ep *Endpoint) assignAddressVersion(ipVer int, ipam ipamapi.Ipam) error {
	var (
		poolID  *string
		address **net.IPNet
		prefAdd net.IP
		progAdd net.IP
	)

	n := ep.getNetwork()
	switch ipVer {
	case 4:
		poolID = &ep.iface.v4PoolID
		address = &ep.iface.addr
		prefAdd = ep.prefAddress
	case 6:
		poolID = &ep.iface.v6PoolID
		address = &ep.iface.addrv6
		prefAdd = ep.prefAddressV6
	default:
		return types.InternalErrorf("incorrect ip version number passed: %d", ipVer)
	}

	ipInfo := n.getIPInfo(ipVer)

	// ipv6 address is not mandatory
	if len(ipInfo) == 0 && ipVer == 6 {
		return nil
	}

	// The address to program may be chosen by the user or by the network driver in one specific
	// case to support backward compatibility with `docker daemon --fixed-cidrv6` use case
	if prefAdd != nil {
		progAdd = prefAdd
	} else if *address != nil {
		progAdd = (*address).IP
	}

	for _, d := range ipInfo {
		if progAdd != nil && !d.Pool.Contains(progAdd) {
			continue
		}
		addr, _, err := ipam.RequestAddress(d.PoolID, progAdd, ep.ipamOptions)
		if err == nil {
			ep.mu.Lock()
			*address = addr
			*poolID = d.PoolID
			ep.mu.Unlock()
			return nil
		}
		if err != ipamapi.ErrNoAvailableIPs || progAdd != nil {
			return err
		}
	}
	if progAdd != nil {
		return types.InvalidParameterErrorf("invalid address %s: It does not belong to any of this network's subnets", prefAdd)
	}
	return fmt.Errorf("no available IPv%d addresses on this network's address pools: %s (%s)", ipVer, n.Name(), n.ID())
}

func (ep *Endpoint) releaseAddress() {
	n := ep.getNetwork()
	if n.hasSpecialDriver() {
		return
	}

	log.G(context.TODO()).Debugf("Releasing addresses for endpoint %s's interface on network %s", ep.Name(), n.Name())

	ipam, _, err := n.getController().getIPAMDriver(n.ipamType)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to retrieve ipam driver to release interface address on delete of endpoint %s (%s): %v", ep.Name(), ep.ID(), err)
		return
	}

	if ep.iface.addr != nil {
		if err := ipam.ReleaseAddress(ep.iface.v4PoolID, ep.iface.addr.IP); err != nil {
			log.G(context.TODO()).Warnf("Failed to release ip address %s on delete of endpoint %s (%s): %v", ep.iface.addr.IP, ep.Name(), ep.ID(), err)
		}
	}

	if ep.iface.addrv6 != nil {
		if err := ipam.ReleaseAddress(ep.iface.v6PoolID, ep.iface.addrv6.IP); err != nil {
			log.G(context.TODO()).Warnf("Failed to release ip address %s on delete of endpoint %s (%s): %v", ep.iface.addrv6.IP, ep.Name(), ep.ID(), err)
		}
	}
}

func (c *Controller) cleanupLocalEndpoints() error {
	// Get used endpoints
	eps := make(map[string]interface{})
	for _, sb := range c.sandboxes {
		for _, ep := range sb.endpoints {
			eps[ep.id] = true
		}
	}
	nl, err := c.getNetworks()
	if err != nil {
		return fmt.Errorf("could not get list of networks: %v", err)
	}

	for _, n := range nl {
		if n.ConfigOnly() {
			continue
		}
		epl, err := n.getEndpointsFromStore()
		if err != nil {
			log.G(context.TODO()).Warnf("Could not get list of endpoints in network %s during endpoint cleanup: %v", n.name, err)
			continue
		}

		for _, ep := range epl {
			if _, ok := eps[ep.id]; ok {
				continue
			}
			log.G(context.TODO()).Infof("Removing stale endpoint %s (%s)", ep.name, ep.id)
			if err := ep.Delete(true); err != nil {
				log.G(context.TODO()).Warnf("Could not delete local endpoint %s during endpoint cleanup: %v", ep.name, err)
			}
		}

		epl, err = n.getEndpointsFromStore()
		if err != nil {
			log.G(context.TODO()).Warnf("Could not get list of endpoints in network %s for count update: %v", n.name, err)
			continue
		}

		epCnt := n.getEpCnt().EndpointCnt()
		if epCnt != uint64(len(epl)) {
			log.G(context.TODO()).Infof("Fixing inconsistent endpoint_cnt for network %s. Expected=%d, Actual=%d", n.name, len(epl), epCnt)
			if err := n.getEpCnt().setCnt(uint64(len(epl))); err != nil {
				log.G(context.TODO()).WithField("network", n.name).WithError(err).Warn("Error while fixing inconsistent endpoint_cnt for network")
			}
		}
	}

	return nil
}
