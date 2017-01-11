package networkallocator

import (
	"fmt"
	"net"

	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drvregistry"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const (
	// DefaultDriver defines the name of the driver to be used by
	// default if a network without any driver name specified is
	// created.
	DefaultDriver = "overlay"
)

// NetworkAllocator acts as the controller for all network related operations
// like managing network and IPAM drivers and also creating and
// deleting networks and the associated resources.
type NetworkAllocator struct {
	// The driver register which manages all internal and external
	// IPAM and network drivers.
	drvRegistry *drvregistry.DrvRegistry

	// The port allocator instance for allocating node ports
	portAllocator *portAllocator

	// Local network state used by NetworkAllocator to do network management.
	networks map[string]*network

	// Allocator state to indicate if allocation has been
	// successfully completed for this service.
	services map[string]struct{}

	// Allocator state to indicate if allocation has been
	// successfully completed for this task.
	tasks map[string]struct{}

	// Allocator state to indicate if allocation has been
	// successfully completed for this node.
	nodes map[string]struct{}
}

// Local in-memory state related to netwok that need to be tracked by NetworkAllocator
type network struct {
	// A local cache of the store object.
	nw *api.Network

	// pools is used to save the internal poolIDs needed when
	// releasing the pool.
	pools map[string]string

	// endpoints is a map of endpoint IP to the poolID from which it
	// was allocated.
	endpoints map[string]string
}

type initializer struct {
	fn    drvregistry.InitFunc
	ntype string
}

// New returns a new NetworkAllocator handle
func New() (*NetworkAllocator, error) {
	na := &NetworkAllocator{
		networks: make(map[string]*network),
		services: make(map[string]struct{}),
		tasks:    make(map[string]struct{}),
		nodes:    make(map[string]struct{}),
	}

	// There are no driver configurations and notification
	// functions as of now.
	reg, err := drvregistry.New(nil, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	if err := initializeDrivers(reg); err != nil {
		return nil, err
	}

	if err = initIPAMDrivers(reg); err != nil {
		return nil, err
	}

	pa, err := newPortAllocator()
	if err != nil {
		return nil, err
	}

	na.portAllocator = pa
	na.drvRegistry = reg
	return na, nil
}

// Allocate allocates all the necessary resources both general
// and driver-specific which may be specified in the NetworkSpec
func (na *NetworkAllocator) Allocate(n *api.Network) error {
	if _, ok := na.networks[n.ID]; ok {
		return fmt.Errorf("network %s already allocated", n.ID)
	}

	pools, err := na.allocatePools(n)
	if err != nil {
		return errors.Wrapf(err, "failed allocating pools and gateway IP for network %s", n.ID)
	}

	if err := na.allocateDriverState(n); err != nil {
		na.freePools(n, pools)
		return errors.Wrapf(err, "failed while allocating driver state for network %s", n.ID)
	}

	na.networks[n.ID] = &network{
		nw:        n,
		pools:     pools,
		endpoints: make(map[string]string),
	}

	return nil
}

func (na *NetworkAllocator) getNetwork(id string) *network {
	return na.networks[id]
}

// Deallocate frees all the general and driver specific resources
// whichs were assigned to the passed network.
func (na *NetworkAllocator) Deallocate(n *api.Network) error {
	localNet := na.getNetwork(n.ID)
	if localNet == nil {
		return fmt.Errorf("could not get networker state for network %s", n.ID)
	}

	if err := na.freeDriverState(n); err != nil {
		return errors.Wrapf(err, "failed to free driver state for network %s", n.ID)
	}

	delete(na.networks, n.ID)
	return na.freePools(n, localNet.pools)
}

// ServiceAllocate allocates all the network resources such as virtual
// IP and ports needed by the service.
func (na *NetworkAllocator) ServiceAllocate(s *api.Service) (err error) {
	if err = na.portAllocator.serviceAllocatePorts(s); err != nil {
		return
	}
	defer func() {
		if err != nil {
			na.ServiceDeallocate(s)
		}
	}()

	if s.Endpoint == nil {
		s.Endpoint = &api.Endpoint{}
	}
	s.Endpoint.Spec = s.Spec.Endpoint.Copy()

	// If ResolutionMode is DNSRR do not try allocating VIPs, but
	// free any VIP from previous state.
	if s.Spec.Endpoint != nil && s.Spec.Endpoint.Mode == api.ResolutionModeDNSRoundRobin {
		if s.Endpoint != nil {
			for _, vip := range s.Endpoint.VirtualIPs {
				if err := na.deallocateVIP(vip); err != nil {
					// don't bail here, deallocate as many as possible.
					log.L.WithError(err).
						WithField("vip.network", vip.NetworkID).
						WithField("vip.addr", vip.Addr).Error("error deallocating vip")
				}
			}

			s.Endpoint.VirtualIPs = nil
		}

		delete(na.services, s.ID)
		return
	}

	// First allocate VIPs for all the pre-populated endpoint attachments
	for _, eAttach := range s.Endpoint.VirtualIPs {
		if err = na.allocateVIP(eAttach); err != nil {
			return
		}
	}

	// Always prefer NetworkAttachmentConfig in the TaskSpec
	specNetworks := s.Spec.Task.Networks
	if len(specNetworks) == 0 && s != nil && len(s.Spec.Networks) != 0 {
		specNetworks = s.Spec.Networks
	}

outer:
	for _, nAttach := range specNetworks {
		for _, vip := range s.Endpoint.VirtualIPs {
			if vip.NetworkID == nAttach.Target {
				continue outer
			}
		}

		vip := &api.Endpoint_VirtualIP{NetworkID: nAttach.Target}
		if err = na.allocateVIP(vip); err != nil {
			return
		}

		s.Endpoint.VirtualIPs = append(s.Endpoint.VirtualIPs, vip)
	}

	na.services[s.ID] = struct{}{}
	return
}

// ServiceDeallocate de-allocates all the network resources such as
// virtual IP and ports associated with the service.
func (na *NetworkAllocator) ServiceDeallocate(s *api.Service) error {
	if s.Endpoint == nil {
		return nil
	}

	for _, vip := range s.Endpoint.VirtualIPs {
		if err := na.deallocateVIP(vip); err != nil {
			// don't bail here, deallocate as many as possible.
			log.L.WithError(err).
				WithField("vip.network", vip.NetworkID).
				WithField("vip.addr", vip.Addr).Error("error deallocating vip")
		}
	}

	na.portAllocator.serviceDeallocatePorts(s)
	delete(na.services, s.ID)

	return nil
}

// IsAllocated returns if the passed network has been allocated or not.
func (na *NetworkAllocator) IsAllocated(n *api.Network) bool {
	_, ok := na.networks[n.ID]
	return ok
}

// IsTaskAllocated returns if the passed task has its network resources allocated or not.
func (na *NetworkAllocator) IsTaskAllocated(t *api.Task) bool {
	// If the task is not found in the allocated set, then it is
	// not allocated.
	if _, ok := na.tasks[t.ID]; !ok {
		return false
	}

	// If Networks is empty there is no way this Task is allocated.
	if len(t.Networks) == 0 {
		return false
	}

	// To determine whether the task has its resources allocated,
	// we just need to look at one network(in case of
	// multi-network attachment).  This is because we make sure we
	// allocate for every network or we allocate for none.

	// If the network is not allocated, the task cannot be allocated.
	localNet, ok := na.networks[t.Networks[0].Network.ID]
	if !ok {
		return false
	}

	// Addresses empty. Task is not allocated.
	if len(t.Networks[0].Addresses) == 0 {
		return false
	}

	// The allocated IP address not found in local endpoint state. Not allocated.
	if _, ok := localNet.endpoints[t.Networks[0].Addresses[0]]; !ok {
		return false
	}

	return true
}

// IsServiceAllocated returns if the passed service has its network resources allocated or not.
func (na *NetworkAllocator) IsServiceAllocated(s *api.Service) bool {
	// If endpoint mode is VIP and allocator does not have the
	// service in VIP allocated set then it is not allocated.
	if (len(s.Spec.Task.Networks) != 0 || len(s.Spec.Networks) != 0) &&
		(s.Spec.Endpoint == nil ||
			s.Spec.Endpoint.Mode == api.ResolutionModeVirtualIP) {
		if _, ok := na.services[s.ID]; !ok {
			return false
		}
	}

	// If the endpoint mode is DNSRR and allocator has the service
	// in VIP allocated set then we return not allocated to make
	// sure the allocator triggers networkallocator to free up the
	// resources if any.
	if s.Spec.Endpoint != nil && s.Spec.Endpoint.Mode == api.ResolutionModeDNSRoundRobin {
		if _, ok := na.services[s.ID]; ok {
			return false
		}
	}

	if (s.Spec.Endpoint != nil && len(s.Spec.Endpoint.Ports) != 0) ||
		(s.Endpoint != nil && len(s.Endpoint.Ports) != 0) {
		return na.portAllocator.isPortsAllocated(s)
	}

	return true
}

// IsNodeAllocated returns if the passed node has its network resources allocated or not.
func (na *NetworkAllocator) IsNodeAllocated(node *api.Node) bool {
	// If the node is not found in the allocated set, then it is
	// not allocated.
	if _, ok := na.nodes[node.ID]; !ok {
		return false
	}

	// If no attachment, not allocated.
	if node.Attachment == nil {
		return false
	}

	// If the network is not allocated, the node cannot be allocated.
	localNet, ok := na.networks[node.Attachment.Network.ID]
	if !ok {
		return false
	}

	// Addresses empty, not allocated.
	if len(node.Attachment.Addresses) == 0 {
		return false
	}

	// The allocated IP address not found in local endpoint state. Not allocated.
	if _, ok := localNet.endpoints[node.Attachment.Addresses[0]]; !ok {
		return false
	}

	return true
}

// AllocateNode allocates the IP addresses for the network to which
// the node is attached.
func (na *NetworkAllocator) AllocateNode(node *api.Node) error {
	if err := na.allocateNetworkIPs(node.Attachment); err != nil {
		return err
	}

	na.nodes[node.ID] = struct{}{}
	return nil
}

// DeallocateNode deallocates the IP addresses for the network to
// which the node is attached.
func (na *NetworkAllocator) DeallocateNode(node *api.Node) error {
	delete(na.nodes, node.ID)
	return na.releaseEndpoints([]*api.NetworkAttachment{node.Attachment})
}

// AllocateTask allocates all the endpoint resources for all the
// networks that a task is attached to.
func (na *NetworkAllocator) AllocateTask(t *api.Task) error {
	for i, nAttach := range t.Networks {
		if err := na.allocateNetworkIPs(nAttach); err != nil {
			if err := na.releaseEndpoints(t.Networks[:i]); err != nil {
				log.G(context.TODO()).WithError(err).Errorf("Failed to release IP addresses while rolling back allocation for task %s network %s", t.ID, nAttach.Network.ID)
			}
			return errors.Wrapf(err, "failed to allocate network IP for task %s network %s", t.ID, nAttach.Network.ID)
		}
	}

	na.tasks[t.ID] = struct{}{}

	return nil
}

// DeallocateTask releases all the endpoint resources for all the
// networks that a task is attached to.
func (na *NetworkAllocator) DeallocateTask(t *api.Task) error {
	delete(na.tasks, t.ID)
	return na.releaseEndpoints(t.Networks)
}

func (na *NetworkAllocator) releaseEndpoints(networks []*api.NetworkAttachment) error {
	for _, nAttach := range networks {
		ipam, _, _, err := na.resolveIPAM(nAttach.Network)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve IPAM while allocating")
		}

		localNet := na.getNetwork(nAttach.Network.ID)
		if localNet == nil {
			return fmt.Errorf("could not find network allocater state for network %s", nAttach.Network.ID)
		}

		// Do not fail and bail out if we fail to release IP
		// address here. Keep going and try releasing as many
		// addresses as possible.
		for _, addr := range nAttach.Addresses {
			// Retrieve the poolID and immediately nuke
			// out the mapping.
			poolID := localNet.endpoints[addr]
			delete(localNet.endpoints, addr)

			ip, _, err := net.ParseCIDR(addr)
			if err != nil {
				log.G(context.TODO()).Errorf("Could not parse IP address %s while releasing", addr)
				continue
			}

			if err := ipam.ReleaseAddress(poolID, ip); err != nil {
				log.G(context.TODO()).WithError(err).Errorf("IPAM failure while releasing IP address %s", addr)
			}
		}

		// Clear out the address list when we are done with
		// this network.
		nAttach.Addresses = nil
	}

	return nil
}

// allocate virtual IP for a single endpoint attachment of the service.
func (na *NetworkAllocator) allocateVIP(vip *api.Endpoint_VirtualIP) error {
	localNet := na.getNetwork(vip.NetworkID)
	if localNet == nil {
		return fmt.Errorf("networkallocator: could not find local network state")
	}

	// If this IP is already allocated in memory we don't need to
	// do anything.
	if _, ok := localNet.endpoints[vip.Addr]; ok {
		return nil
	}

	ipam, _, _, err := na.resolveIPAM(localNet.nw)
	if err != nil {
		return errors.Wrap(err, "failed to resolve IPAM while allocating")
	}

	var addr net.IP
	if vip.Addr != "" {
		var err error

		addr, _, err = net.ParseCIDR(vip.Addr)
		if err != nil {
			return err
		}
	}

	for _, poolID := range localNet.pools {
		ip, _, err := ipam.RequestAddress(poolID, addr, nil)
		if err != nil && err != ipamapi.ErrNoAvailableIPs && err != ipamapi.ErrIPOutOfRange {
			return errors.Wrap(err, "could not allocate VIP from IPAM")
		}

		// If we got an address then we are done.
		if err == nil {
			ipStr := ip.String()
			localNet.endpoints[ipStr] = poolID
			vip.Addr = ipStr
			return nil
		}
	}

	return errors.New("could not find an available IP while allocating VIP")
}

func (na *NetworkAllocator) deallocateVIP(vip *api.Endpoint_VirtualIP) error {
	localNet := na.getNetwork(vip.NetworkID)
	if localNet == nil {
		return errors.New("networkallocator: could not find local network state")
	}

	ipam, _, _, err := na.resolveIPAM(localNet.nw)
	if err != nil {
		return errors.Wrap(err, "failed to resolve IPAM while allocating")
	}

	// Retrieve the poolID and immediately nuke
	// out the mapping.
	poolID := localNet.endpoints[vip.Addr]
	delete(localNet.endpoints, vip.Addr)

	ip, _, err := net.ParseCIDR(vip.Addr)
	if err != nil {
		log.G(context.TODO()).Errorf("Could not parse VIP address %s while releasing", vip.Addr)
		return err
	}

	if err := ipam.ReleaseAddress(poolID, ip); err != nil {
		log.G(context.TODO()).WithError(err).Errorf("IPAM failure while releasing VIP address %s", vip.Addr)
		return err
	}

	return nil
}

// allocate the IP addresses for a single network attachment of the task.
func (na *NetworkAllocator) allocateNetworkIPs(nAttach *api.NetworkAttachment) error {
	var ip *net.IPNet

	ipam, _, _, err := na.resolveIPAM(nAttach.Network)
	if err != nil {
		return errors.Wrap(err, "failed to resolve IPAM while allocating")
	}

	localNet := na.getNetwork(nAttach.Network.ID)
	if localNet == nil {
		return fmt.Errorf("could not find network allocator state for network %s", nAttach.Network.ID)
	}

	addresses := nAttach.Addresses
	if len(addresses) == 0 {
		addresses = []string{""}
	}

	for i, rawAddr := range addresses {
		var addr net.IP
		if rawAddr != "" {
			var err error
			addr, _, err = net.ParseCIDR(rawAddr)
			if err != nil {
				addr = net.ParseIP(rawAddr)

				if addr == nil {
					return errors.Wrapf(err, "could not parse address string %s", rawAddr)
				}
			}
		}

		for _, poolID := range localNet.pools {
			var err error

			ip, _, err = ipam.RequestAddress(poolID, addr, nil)
			if err != nil && err != ipamapi.ErrNoAvailableIPs && err != ipamapi.ErrIPOutOfRange {
				return errors.Wrap(err, "could not allocate IP from IPAM")
			}

			// If we got an address then we are done.
			if err == nil {
				ipStr := ip.String()
				localNet.endpoints[ipStr] = poolID
				addresses[i] = ipStr
				nAttach.Addresses = addresses
				return nil
			}
		}
	}

	return errors.New("could not find an available IP")
}

func (na *NetworkAllocator) freeDriverState(n *api.Network) error {
	d, _, err := na.resolveDriver(n)
	if err != nil {
		return err
	}

	return d.NetworkFree(n.ID)
}

func (na *NetworkAllocator) allocateDriverState(n *api.Network) error {
	d, dName, err := na.resolveDriver(n)
	if err != nil {
		return err
	}

	options := make(map[string]string)
	// reconcile the driver specific options from the network spec
	// and from the operational state retrieved from the store
	if n.Spec.DriverConfig != nil {
		for k, v := range n.Spec.DriverConfig.Options {
			options[k] = v
		}
	}
	if n.DriverState != nil {
		for k, v := range n.DriverState.Options {
			options[k] = v
		}
	}

	// Construct IPAM data for driver consumption.
	ipv4Data := make([]driverapi.IPAMData, 0, len(n.IPAM.Configs))
	for _, ic := range n.IPAM.Configs {
		if ic.Family == api.IPAMConfig_IPV6 {
			continue
		}

		_, subnet, err := net.ParseCIDR(ic.Subnet)
		if err != nil {
			return errors.Wrapf(err, "error parsing subnet %s while allocating driver state", ic.Subnet)
		}

		gwIP := net.ParseIP(ic.Gateway)
		gwNet := &net.IPNet{
			IP:   gwIP,
			Mask: subnet.Mask,
		}

		data := driverapi.IPAMData{
			Pool:    subnet,
			Gateway: gwNet,
		}

		ipv4Data = append(ipv4Data, data)
	}

	ds, err := d.NetworkAllocate(n.ID, options, ipv4Data, nil)
	if err != nil {
		return err
	}

	// Update network object with the obtained driver state.
	n.DriverState = &api.Driver{
		Name:    dName,
		Options: ds,
	}

	return nil
}

// Resolve network driver
func (na *NetworkAllocator) resolveDriver(n *api.Network) (driverapi.Driver, string, error) {
	dName := DefaultDriver
	if n.Spec.DriverConfig != nil && n.Spec.DriverConfig.Name != "" {
		dName = n.Spec.DriverConfig.Name
	}

	d, drvcap := na.drvRegistry.Driver(dName)
	if d == nil {
		var err error
		err = na.loadDriver(dName)
		if err != nil {
			return nil, "", err
		}

		d, drvcap = na.drvRegistry.Driver(dName)
		if d == nil {
			return nil, "", fmt.Errorf("could not resolve network driver %s", dName)
		}

	}

	if drvcap.DataScope != datastore.GlobalScope {
		return nil, "", fmt.Errorf("swarm can allocate network resources only for global scoped networks. network driver (%s) is scoped %s", dName, drvcap.DataScope)
	}

	return d, dName, nil
}

func (na *NetworkAllocator) loadDriver(name string) error {
	_, err := plugins.Get(name, driverapi.NetworkPluginEndpointType)
	return err
}

// Resolve the IPAM driver
func (na *NetworkAllocator) resolveIPAM(n *api.Network) (ipamapi.Ipam, string, map[string]string, error) {
	dName := ipamapi.DefaultIPAM
	if n.Spec.IPAM != nil && n.Spec.IPAM.Driver != nil && n.Spec.IPAM.Driver.Name != "" {
		dName = n.Spec.IPAM.Driver.Name
	}

	var dOptions map[string]string
	if n.Spec.IPAM != nil && n.Spec.IPAM.Driver != nil && len(n.Spec.IPAM.Driver.Options) != 0 {
		dOptions = n.Spec.IPAM.Driver.Options
	}

	ipam, _ := na.drvRegistry.IPAM(dName)
	if ipam == nil {
		return nil, "", nil, fmt.Errorf("could not resolve IPAM driver %s", dName)
	}

	return ipam, dName, dOptions, nil
}

func (na *NetworkAllocator) freePools(n *api.Network, pools map[string]string) error {
	ipam, _, _, err := na.resolveIPAM(n)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve IPAM while freeing pools for network %s", n.ID)
	}

	releasePools(ipam, n.IPAM.Configs, pools)
	return nil
}

func releasePools(ipam ipamapi.Ipam, icList []*api.IPAMConfig, pools map[string]string) {
	for _, ic := range icList {
		if err := ipam.ReleaseAddress(pools[ic.Subnet], net.ParseIP(ic.Gateway)); err != nil {
			log.G(context.TODO()).WithError(err).Errorf("Failed to release address %s", ic.Subnet)
		}
	}

	for k, p := range pools {
		if err := ipam.ReleasePool(p); err != nil {
			log.G(context.TODO()).WithError(err).Errorf("Failed to release pool %s", k)
		}
	}
}

func (na *NetworkAllocator) allocatePools(n *api.Network) (map[string]string, error) {
	ipam, dName, dOptions, err := na.resolveIPAM(n)
	if err != nil {
		return nil, err
	}

	// We don't support user defined address spaces yet so just
	// retrive default address space names for the driver.
	_, asName, err := na.drvRegistry.IPAMDefaultAddressSpaces(dName)
	if err != nil {
		return nil, err
	}

	pools := make(map[string]string)

	if n.Spec.IPAM == nil {
		n.Spec.IPAM = &api.IPAMOptions{}
	}

	ipamConfigs := make([]*api.IPAMConfig, len(n.Spec.IPAM.Configs))
	copy(ipamConfigs, n.Spec.IPAM.Configs)

	// If there is non-nil IPAM state always prefer those subnet
	// configs over Spec configs.
	if n.IPAM != nil {
		ipamConfigs = n.IPAM.Configs
	}

	// Append an empty slot for subnet allocation if there are no
	// IPAM configs from either spec or state.
	if len(ipamConfigs) == 0 {
		ipamConfigs = append(ipamConfigs, &api.IPAMConfig{Family: api.IPAMConfig_IPV4})
	}

	// Update the runtime IPAM configurations with initial state
	n.IPAM = &api.IPAMOptions{
		Driver:  &api.Driver{Name: dName, Options: dOptions},
		Configs: ipamConfigs,
	}

	for i, ic := range ipamConfigs {
		poolID, poolIP, _, err := ipam.RequestPool(asName, ic.Subnet, ic.Range, dOptions, false)
		if err != nil {
			// Rollback by releasing all the resources allocated so far.
			releasePools(ipam, ipamConfigs[:i], pools)
			return nil, err
		}
		pools[poolIP.String()] = poolID

		gwIP, _, err := ipam.RequestAddress(poolID, net.ParseIP(ic.Gateway), nil)
		if err != nil {
			// Rollback by releasing all the resources allocated so far.
			releasePools(ipam, ipamConfigs[:i], pools)
			return nil, err
		}

		if ic.Subnet == "" {
			ic.Subnet = poolIP.String()
		}

		if ic.Gateway == "" {
			ic.Gateway = gwIP.IP.String()
		}

	}

	return pools, nil
}

func initializeDrivers(reg *drvregistry.DrvRegistry) error {
	for _, i := range getInitializers() {
		if err := reg.AddDriver(i.ntype, i.fn, nil); err != nil {
			return err
		}
	}
	return nil
}
