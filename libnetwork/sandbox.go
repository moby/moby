package libnetwork

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/libnetwork/etchosts"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
)

// SandboxOption is an option setter function type used to pass various options to
// NewNetContainer method. The various setter functions of type SandboxOption are
// provided by libnetwork, they look like ContainerOptionXXXX(...)
type SandboxOption func(sb *Sandbox)

func (sb *Sandbox) processOptions(options ...SandboxOption) {
	for _, opt := range options {
		if opt != nil {
			opt(sb)
		}
	}
}

// Sandbox provides the control over the network container entity.
// It is a one to one mapping with the container.
type Sandbox struct {
	id                 string
	containerID        string
	config             containerConfig
	extDNS             []extDNSEntry
	osSbox             osl.Sandbox
	controller         *Controller
	resolver           *Resolver
	resolverOnce       sync.Once
	endpoints          []*Endpoint
	epPriority         map[string]int
	populatedEndpoints map[string]struct{}
	joinLeaveDone      chan struct{}
	dbIndex            uint64
	dbExists           bool
	isStub             bool
	inDelete           bool
	ingress            bool
	ndotsSet           bool
	oslTypes           []osl.SandboxType // slice of properties of this sandbox
	loadBalancerNID    string            // NID that this SB is a load balancer for
	mu                 sync.Mutex
	// This mutex is used to serialize service related operation for an endpoint
	// The lock is here because the endpoint is saved into the store so is not unique
	service sync.Mutex
}

// These are the container configs used to customize container /etc/hosts file.
type hostsPathConfig struct {
	hostName        string
	domainName      string
	hostsPath       string
	originHostsPath string
	extraHosts      []extraHost
	parentUpdates   []parentUpdate
}

type parentUpdate struct {
	cid  string
	name string
	ip   string
}

type extraHost struct {
	name string
	IP   string
}

// These are the container configs used to customize container /etc/resolv.conf file.
type resolvConfPathConfig struct {
	resolvConfPath       string
	originResolvConfPath string
	resolvConfHashFile   string
	dnsList              []string
	dnsSearchList        []string
	dnsOptionsList       []string
}

type containerConfig struct {
	hostsPathConfig
	resolvConfPathConfig
	generic           map[string]interface{}
	useDefaultSandBox bool
	useExternalKey    bool
	exposedPorts      []types.TransportPort
}

const (
	resolverIPSandbox = "127.0.0.11"
)

// ID returns the ID of the sandbox.
func (sb *Sandbox) ID() string {
	return sb.id
}

// ContainerID returns the container id associated to this sandbox.
func (sb *Sandbox) ContainerID() string {
	return sb.containerID
}

// Key returns the sandbox's key.
func (sb *Sandbox) Key() string {
	if sb.config.useDefaultSandBox {
		return osl.GenerateKey("default")
	}
	return osl.GenerateKey(sb.id)
}

// Labels returns the sandbox's labels.
func (sb *Sandbox) Labels() map[string]interface{} {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	opts := make(map[string]interface{}, len(sb.config.generic))
	for k, v := range sb.config.generic {
		opts[k] = v
	}
	return opts
}

// Statistics retrieves the interfaces' statistics for the sandbox.
func (sb *Sandbox) Statistics() (map[string]*types.InterfaceStatistics, error) {
	m := make(map[string]*types.InterfaceStatistics)

	sb.mu.Lock()
	osb := sb.osSbox
	sb.mu.Unlock()
	if osb == nil {
		return m, nil
	}

	var err error
	for _, i := range osb.Info().Interfaces() {
		if m[i.DstName()], err = i.Statistics(); err != nil {
			return m, err
		}
	}

	return m, nil
}

// Delete destroys this container after detaching it from all connected endpoints.
func (sb *Sandbox) Delete() error {
	return sb.delete(false)
}

func (sb *Sandbox) delete(force bool) error {
	sb.mu.Lock()
	if sb.inDelete {
		sb.mu.Unlock()
		return types.ForbiddenErrorf("another sandbox delete in progress")
	}
	// Set the inDelete flag. This will ensure that we don't
	// update the store until we have completed all the endpoint
	// leaves and deletes. And when endpoint leaves and deletes
	// are completed then we can finally delete the sandbox object
	// altogether from the data store. If the daemon exits
	// ungracefully in the middle of a sandbox delete this way we
	// will have all the references to the endpoints in the
	// sandbox so that we can clean them up when we restart
	sb.inDelete = true
	sb.mu.Unlock()

	c := sb.controller

	// Detach from all endpoints
	retain := false
	for _, ep := range sb.Endpoints() {
		// gw network endpoint detach and removal are automatic
		if ep.endpointInGWNetwork() && !force {
			continue
		}
		// Retain the sanbdox if we can't obtain the network from store.
		if _, err := c.getNetworkFromStore(ep.getNetwork().ID()); err != nil {
			if c.isDistributedControl() {
				retain = true
			}
			logrus.Warnf("Failed getting network for ep %s during sandbox %s delete: %v", ep.ID(), sb.ID(), err)
			continue
		}

		if !force {
			if err := ep.Leave(sb); err != nil {
				logrus.Warnf("Failed detaching sandbox %s from endpoint %s: %v\n", sb.ID(), ep.ID(), err)
			}
		}

		if err := ep.Delete(force); err != nil {
			logrus.Warnf("Failed deleting endpoint %s: %v\n", ep.ID(), err)
		}
	}

	if retain {
		sb.mu.Lock()
		sb.inDelete = false
		sb.mu.Unlock()
		return fmt.Errorf("could not cleanup all the endpoints in container %s / sandbox %s", sb.containerID, sb.id)
	}
	// Container is going away. Path cache in etchosts is most
	// likely not required any more. Drop it.
	etchosts.Drop(sb.config.hostsPath)

	if sb.resolver != nil {
		sb.resolver.Stop()
	}

	if sb.osSbox != nil && !sb.config.useDefaultSandBox {
		if err := sb.osSbox.Destroy(); err != nil {
			logrus.WithError(err).Warn("error destroying network sandbox")
		}
	}

	if err := sb.storeDelete(); err != nil {
		logrus.Warnf("Failed to delete sandbox %s from store: %v", sb.ID(), err)
	}

	c.mu.Lock()
	if sb.ingress {
		c.ingressSandbox = nil
	}
	delete(c.sandboxes, sb.ID())
	c.mu.Unlock()

	return nil
}

// Rename changes the name of all attached Endpoints.
func (sb *Sandbox) Rename(name string) error {
	var err error

	for _, ep := range sb.Endpoints() {
		if ep.endpointInGWNetwork() {
			continue
		}

		oldName := ep.Name()
		lEp := ep
		if err = ep.rename(name); err != nil {
			break
		}

		defer func() {
			if err != nil {
				if err2 := lEp.rename(oldName); err2 != nil {
					logrus.WithField("old", oldName).WithField("origError", err).WithError(err2).Error("error renaming sandbox")
				}
			}
		}()
	}

	return err
}

// Refresh leaves all the endpoints, resets and re-applies the options,
// re-joins all the endpoints without destroying the osl sandbox
func (sb *Sandbox) Refresh(options ...SandboxOption) error {
	// Store connected endpoints
	epList := sb.Endpoints()

	// Detach from all endpoints
	for _, ep := range epList {
		if err := ep.Leave(sb); err != nil {
			logrus.Warnf("Failed detaching sandbox %s from endpoint %s: %v\n", sb.ID(), ep.ID(), err)
		}
	}

	// Re-apply options
	sb.config = containerConfig{}
	sb.processOptions(options...)

	// Setup discovery files
	if err := sb.setupResolutionFiles(); err != nil {
		return err
	}

	// Re-connect to all endpoints
	for _, ep := range epList {
		if err := ep.Join(sb); err != nil {
			logrus.Warnf("Failed attach sandbox %s to endpoint %s: %v\n", sb.ID(), ep.ID(), err)
		}
	}

	return nil
}

func (sb *Sandbox) MarshalJSON() ([]byte, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// We are just interested in the container ID. This can be expanded to include all of containerInfo if there is a need
	return json.Marshal(sb.id)
}

func (sb *Sandbox) UnmarshalJSON(b []byte) (err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	var id string
	if err := json.Unmarshal(b, &id); err != nil {
		return err
	}
	sb.id = id
	return nil
}

// Endpoints returns all the endpoints connected to the sandbox.
func (sb *Sandbox) Endpoints() []*Endpoint {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	eps := make([]*Endpoint, len(sb.endpoints))
	copy(eps, sb.endpoints)

	return eps
}

func (sb *Sandbox) addEndpoint(ep *Endpoint) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	l := len(sb.endpoints)
	i := sort.Search(l, func(j int) bool {
		return ep.Less(sb.endpoints[j])
	})

	sb.endpoints = append(sb.endpoints, nil)
	copy(sb.endpoints[i+1:], sb.endpoints[i:])
	sb.endpoints[i] = ep
}

func (sb *Sandbox) removeEndpoint(ep *Endpoint) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.removeEndpointRaw(ep)
}

func (sb *Sandbox) removeEndpointRaw(ep *Endpoint) {
	for i, e := range sb.endpoints {
		if e == ep {
			sb.endpoints = append(sb.endpoints[:i], sb.endpoints[i+1:]...)
			return
		}
	}
}

func (sb *Sandbox) getEndpoint(id string) *Endpoint {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	for _, ep := range sb.endpoints {
		if ep.id == id {
			return ep
		}
	}

	return nil
}

func (sb *Sandbox) updateGateway(ep *Endpoint) error {
	sb.mu.Lock()
	osSbox := sb.osSbox
	sb.mu.Unlock()
	if osSbox == nil {
		return nil
	}
	osSbox.UnsetGateway()     //nolint:errcheck
	osSbox.UnsetGatewayIPv6() //nolint:errcheck

	if ep == nil {
		return nil
	}

	ep.mu.Lock()
	joinInfo := ep.joinInfo
	ep.mu.Unlock()

	if err := osSbox.SetGateway(joinInfo.gw); err != nil {
		return fmt.Errorf("failed to set gateway while updating gateway: %v", err)
	}

	if err := osSbox.SetGatewayIPv6(joinInfo.gw6); err != nil {
		return fmt.Errorf("failed to set IPv6 gateway while updating gateway: %v", err)
	}

	return nil
}

func (sb *Sandbox) HandleQueryResp(name string, ip net.IP) {
	for _, ep := range sb.Endpoints() {
		n := ep.getNetwork()
		n.HandleQueryResp(name, ip)
	}
}

func (sb *Sandbox) ResolveIP(ip string) string {
	var svc string
	logrus.Debugf("IP To resolve %v", ip)

	for _, ep := range sb.Endpoints() {
		n := ep.getNetwork()
		svc = n.ResolveIP(ip)
		if len(svc) != 0 {
			return svc
		}
	}

	return svc
}

func (sb *Sandbox) ExecFunc(f func()) error {
	sb.mu.Lock()
	osSbox := sb.osSbox
	sb.mu.Unlock()
	if osSbox != nil {
		return osSbox.InvokeFunc(f)
	}
	return fmt.Errorf("osl sandbox unavailable in ExecFunc for %v", sb.ContainerID())
}

// ResolveService returns all the backend details about the containers or hosts
// backing a service. Its purpose is to satisfy an SRV query.
func (sb *Sandbox) ResolveService(name string) ([]*net.SRV, []net.IP) {
	srv := []*net.SRV{}
	ip := []net.IP{}

	logrus.Debugf("Service name To resolve: %v", name)

	// There are DNS implementations that allow SRV queries for names not in
	// the format defined by RFC 2782. Hence specific validations checks are
	// not done
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		return nil, nil
	}

	for _, ep := range sb.Endpoints() {
		n := ep.getNetwork()

		srv, ip = n.ResolveService(name)
		if len(srv) > 0 {
			break
		}
	}
	return srv, ip
}

func getDynamicNwEndpoints(epList []*Endpoint) []*Endpoint {
	eps := []*Endpoint{}
	for _, ep := range epList {
		n := ep.getNetwork()
		if n.dynamic && !n.ingress {
			eps = append(eps, ep)
		}
	}
	return eps
}

func getIngressNwEndpoint(epList []*Endpoint) *Endpoint {
	for _, ep := range epList {
		n := ep.getNetwork()
		if n.ingress {
			return ep
		}
	}
	return nil
}

func getLocalNwEndpoints(epList []*Endpoint) []*Endpoint {
	eps := []*Endpoint{}
	for _, ep := range epList {
		n := ep.getNetwork()
		if !n.dynamic && !n.ingress {
			eps = append(eps, ep)
		}
	}
	return eps
}

func (sb *Sandbox) ResolveName(name string, ipType int) ([]net.IP, bool) {
	// Embedded server owns the docker network domain. Resolution should work
	// for both container_name and container_name.network_name
	// We allow '.' in service name and network name. For a name a.b.c.d the
	// following have to tried;
	// {a.b.c.d in the networks container is connected to}
	// {a.b.c in network d},
	// {a.b in network c.d},
	// {a in network b.c.d},

	logrus.Debugf("Name To resolve: %v", name)
	name = strings.TrimSuffix(name, ".")
	reqName := []string{name}
	networkName := []string{""}

	if strings.Contains(name, ".") {
		var i int
		dup := name
		for {
			if i = strings.LastIndex(dup, "."); i == -1 {
				break
			}
			networkName = append(networkName, name[i+1:])
			reqName = append(reqName, name[:i])

			dup = dup[:i]
		}
	}

	epList := sb.Endpoints()

	// In swarm mode services with exposed ports are connected to user overlay
	// network, ingress network and docker_gwbridge network. Name resolution
	// should prioritize returning the VIP/IPs on user overlay network.
	newList := []*Endpoint{}
	if !sb.controller.isDistributedControl() {
		newList = append(newList, getDynamicNwEndpoints(epList)...)
		ingressEP := getIngressNwEndpoint(epList)
		if ingressEP != nil {
			newList = append(newList, ingressEP)
		}
		newList = append(newList, getLocalNwEndpoints(epList)...)
		epList = newList
	}

	for i := 0; i < len(reqName); i++ {
		// First check for local container alias
		ip, ipv6Miss := sb.resolveName(reqName[i], networkName[i], epList, true, ipType)
		if ip != nil {
			return ip, false
		}
		if ipv6Miss {
			return ip, ipv6Miss
		}

		// Resolve the actual container name
		ip, ipv6Miss = sb.resolveName(reqName[i], networkName[i], epList, false, ipType)
		if ip != nil {
			return ip, false
		}
		if ipv6Miss {
			return ip, ipv6Miss
		}
	}
	return nil, false
}

func (sb *Sandbox) resolveName(req string, networkName string, epList []*Endpoint, alias bool, ipType int) ([]net.IP, bool) {
	var ipv6Miss bool

	for _, ep := range epList {
		name := req
		n := ep.getNetwork()

		if networkName != "" && networkName != n.Name() {
			continue
		}

		if alias {
			if ep.aliases == nil {
				continue
			}

			var ok bool
			ep.mu.Lock()
			name, ok = ep.aliases[req]
			ep.mu.Unlock()
			if !ok {
				continue
			}
		} else {
			// If it is a regular lookup and if the requested name is an alias
			// don't perform a svc lookup for this endpoint.
			ep.mu.Lock()
			if _, ok := ep.aliases[req]; ok {
				ep.mu.Unlock()
				continue
			}
			ep.mu.Unlock()
		}

		ip, miss := n.ResolveName(name, ipType)

		if ip != nil {
			return ip, false
		}

		if miss {
			ipv6Miss = miss
		}
	}
	return nil, ipv6Miss
}

// SetKey updates the Sandbox Key.
func (sb *Sandbox) SetKey(basePath string) error {
	start := time.Now()
	defer func() {
		logrus.Debugf("sandbox set key processing took %s for container %s", time.Since(start), sb.ContainerID())
	}()

	if basePath == "" {
		return types.BadRequestErrorf("invalid sandbox key")
	}

	sb.mu.Lock()
	if sb.inDelete {
		sb.mu.Unlock()
		return types.ForbiddenErrorf("failed to SetKey: sandbox %q delete in progress", sb.id)
	}
	oldosSbox := sb.osSbox
	sb.mu.Unlock()

	if oldosSbox != nil {
		// If we already have an OS sandbox, release the network resources from that
		// and destroy the OS snab. We are moving into a new home further down. Note that none
		// of the network resources gets destroyed during the move.
		sb.releaseOSSbox()
	}

	osSbox, err := osl.GetSandboxForExternalKey(basePath, sb.Key())
	if err != nil {
		return err
	}

	sb.mu.Lock()
	sb.osSbox = osSbox
	sb.mu.Unlock()

	// If the resolver was setup before stop it and set it up in the
	// new osl sandbox.
	if oldosSbox != nil && sb.resolver != nil {
		sb.resolver.Stop()

		if err := sb.osSbox.InvokeFunc(sb.resolver.SetupFunc(0)); err == nil {
			if err := sb.resolver.Start(); err != nil {
				logrus.Errorf("Resolver Start failed for container %s, %q", sb.ContainerID(), err)
			}
		} else {
			logrus.Errorf("Resolver Setup Function failed for container %s, %q", sb.ContainerID(), err)
		}
	}

	for _, ep := range sb.Endpoints() {
		if err = sb.populateNetworkResources(ep); err != nil {
			return err
		}
	}
	return nil
}

// EnableService makes a managed container's service available by adding the
// endpoint to the service load balancer and service discovery.
func (sb *Sandbox) EnableService() (err error) {
	logrus.Debugf("EnableService %s START", sb.containerID)
	defer func() {
		if err != nil {
			if err2 := sb.DisableService(); err2 != nil {
				logrus.WithError(err2).WithField("origError", err).Error("Error while disabling service after original error")
			}
		}
	}()
	for _, ep := range sb.Endpoints() {
		if !ep.isServiceEnabled() {
			if err := ep.addServiceInfoToCluster(sb); err != nil {
				return fmt.Errorf("could not update state for endpoint %s into cluster: %v", ep.Name(), err)
			}
			ep.enableService()
		}
	}
	logrus.Debugf("EnableService %s DONE", sb.containerID)
	return nil
}

// DisableService removes a managed container's endpoints from the load balancer
// and service discovery.
func (sb *Sandbox) DisableService() (err error) {
	logrus.Debugf("DisableService %s START", sb.containerID)
	failedEps := []string{}
	defer func() {
		if len(failedEps) > 0 {
			err = fmt.Errorf("failed to disable service on sandbox:%s, for endpoints %s", sb.ID(), strings.Join(failedEps, ","))
		}
	}()
	for _, ep := range sb.Endpoints() {
		if ep.isServiceEnabled() {
			if err := ep.deleteServiceInfoFromCluster(sb, false, "DisableService"); err != nil {
				failedEps = append(failedEps, ep.Name())
				logrus.Warnf("failed update state for endpoint %s into cluster: %v", ep.Name(), err)
			}
			ep.disableService()
		}
	}
	logrus.Debugf("DisableService %s DONE", sb.containerID)
	return nil
}

func releaseOSSboxResources(osSbox osl.Sandbox, ep *Endpoint) {
	for _, i := range osSbox.Info().Interfaces() {
		// Only remove the interfaces owned by this endpoint from the sandbox.
		if ep.hasInterface(i.SrcName()) {
			if err := i.Remove(); err != nil {
				logrus.Debugf("Remove interface %s failed: %v", i.SrcName(), err)
			}
		}
	}

	ep.mu.Lock()
	joinInfo := ep.joinInfo
	vip := ep.virtualIP
	lbModeIsDSR := ep.network.loadBalancerMode == loadBalancerModeDSR
	ep.mu.Unlock()

	if len(vip) > 0 && lbModeIsDSR {
		ipNet := &net.IPNet{IP: vip, Mask: net.CIDRMask(32, 32)}
		if err := osSbox.RemoveAliasIP(osSbox.GetLoopbackIfaceName(), ipNet); err != nil {
			logrus.WithError(err).Debugf("failed to remove virtual ip %v to loopback", ipNet)
		}
	}

	if joinInfo == nil {
		return
	}

	// Remove non-interface routes.
	for _, r := range joinInfo.StaticRoutes {
		if err := osSbox.RemoveStaticRoute(r); err != nil {
			logrus.Debugf("Remove route failed: %v", err)
		}
	}
}

func (sb *Sandbox) releaseOSSbox() {
	sb.mu.Lock()
	osSbox := sb.osSbox
	sb.osSbox = nil
	sb.mu.Unlock()

	if osSbox == nil {
		return
	}

	for _, ep := range sb.Endpoints() {
		releaseOSSboxResources(osSbox, ep)
	}

	if err := osSbox.Destroy(); err != nil {
		logrus.WithError(err).Error("Error destroying os sandbox")
	}
}

func (sb *Sandbox) restoreOslSandbox() error {
	var routes []*types.StaticRoute

	// restore osl sandbox
	Ifaces := make(map[string][]osl.IfaceOption)
	for _, ep := range sb.endpoints {
		ep.mu.Lock()
		joinInfo := ep.joinInfo
		i := ep.iface
		ep.mu.Unlock()

		if i == nil {
			logrus.Errorf("error restoring endpoint %s for container %s", ep.Name(), sb.ContainerID())
			continue
		}

		ifaceOptions := []osl.IfaceOption{
			sb.osSbox.InterfaceOptions().Address(i.addr),
			sb.osSbox.InterfaceOptions().Routes(i.routes),
		}
		if i.addrv6 != nil && i.addrv6.IP.To16() != nil {
			ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().AddressIPv6(i.addrv6))
		}
		if i.mac != nil {
			ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().MacAddress(i.mac))
		}
		if len(i.llAddrs) != 0 {
			ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().LinkLocalAddresses(i.llAddrs))
		}
		Ifaces[i.srcName+i.dstPrefix] = ifaceOptions
		if joinInfo != nil {
			routes = append(routes, joinInfo.StaticRoutes...)
		}
		if ep.needResolver() {
			sb.startResolver(true)
		}
	}

	gwep := sb.getGatewayEndpoint()
	if gwep == nil {
		return nil
	}

	// restore osl sandbox
	return sb.osSbox.Restore(Ifaces, routes, gwep.joinInfo.gw, gwep.joinInfo.gw6)
}

func (sb *Sandbox) populateNetworkResources(ep *Endpoint) error {
	sb.mu.Lock()
	if sb.osSbox == nil {
		sb.mu.Unlock()
		return nil
	}
	inDelete := sb.inDelete
	sb.mu.Unlock()

	ep.mu.Lock()
	joinInfo := ep.joinInfo
	i := ep.iface
	lbModeIsDSR := ep.network.loadBalancerMode == loadBalancerModeDSR
	ep.mu.Unlock()

	if ep.needResolver() {
		sb.startResolver(false)
	}

	if i != nil && i.srcName != "" {
		var ifaceOptions []osl.IfaceOption

		ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().Address(i.addr), sb.osSbox.InterfaceOptions().Routes(i.routes))
		if i.addrv6 != nil && i.addrv6.IP.To16() != nil {
			ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().AddressIPv6(i.addrv6))
		}
		if len(i.llAddrs) != 0 {
			ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().LinkLocalAddresses(i.llAddrs))
		}
		if i.mac != nil {
			ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().MacAddress(i.mac))
		}

		if err := sb.osSbox.AddInterface(i.srcName, i.dstPrefix, ifaceOptions...); err != nil {
			return fmt.Errorf("failed to add interface %s to sandbox: %v", i.srcName, err)
		}

		if len(ep.virtualIP) > 0 && lbModeIsDSR {
			if sb.loadBalancerNID == "" {
				if err := sb.osSbox.DisableARPForVIP(i.srcName); err != nil {
					return fmt.Errorf("failed disable ARP for VIP: %v", err)
				}
			}
			ipNet := &net.IPNet{IP: ep.virtualIP, Mask: net.CIDRMask(32, 32)}
			if err := sb.osSbox.AddAliasIP(sb.osSbox.GetLoopbackIfaceName(), ipNet); err != nil {
				return fmt.Errorf("failed to add virtual ip %v to loopback: %v", ipNet, err)
			}
		}
	}

	if joinInfo != nil {
		// Set up non-interface routes.
		for _, r := range joinInfo.StaticRoutes {
			if err := sb.osSbox.AddStaticRoute(r); err != nil {
				return fmt.Errorf("failed to add static route %s: %v", r.Destination.String(), err)
			}
		}
	}

	if ep == sb.getGatewayEndpoint() {
		if err := sb.updateGateway(ep); err != nil {
			return err
		}
	}

	// Make sure to add the endpoint to the populated endpoint set
	// before populating loadbalancers.
	sb.mu.Lock()
	sb.populatedEndpoints[ep.ID()] = struct{}{}
	sb.mu.Unlock()

	// Populate load balancer only after updating all the other
	// information including gateway and other routes so that
	// loadbalancers are populated all the network state is in
	// place in the sandbox.
	sb.populateLoadBalancers(ep)

	// Only update the store if we did not come here as part of
	// sandbox delete. If we came here as part of delete then do
	// not bother updating the store. The sandbox object will be
	// deleted anyway
	if !inDelete {
		return sb.storeUpdate()
	}

	return nil
}

func (sb *Sandbox) clearNetworkResources(origEp *Endpoint) error {
	ep := sb.getEndpoint(origEp.id)
	if ep == nil {
		return fmt.Errorf("could not find the sandbox endpoint data for endpoint %s",
			origEp.id)
	}

	sb.mu.Lock()
	osSbox := sb.osSbox
	inDelete := sb.inDelete
	sb.mu.Unlock()
	if osSbox != nil {
		releaseOSSboxResources(osSbox, ep)
	}

	sb.mu.Lock()
	delete(sb.populatedEndpoints, ep.ID())

	if len(sb.endpoints) == 0 {
		// sb.endpoints should never be empty and this is unexpected error condition
		// We log an error message to note this down for debugging purposes.
		logrus.Errorf("No endpoints in sandbox while trying to remove endpoint %s", ep.Name())
		sb.mu.Unlock()
		return nil
	}

	var (
		gwepBefore, gwepAfter *Endpoint
		index                 = -1
	)
	for i, e := range sb.endpoints {
		if e == ep {
			index = i
		}
		if len(e.Gateway()) > 0 && gwepBefore == nil {
			gwepBefore = e
		}
		if index != -1 && gwepBefore != nil {
			break
		}
	}

	if index == -1 {
		logrus.Warnf("Endpoint %s has already been deleted", ep.Name())
		sb.mu.Unlock()
		return nil
	}

	sb.removeEndpointRaw(ep)
	for _, e := range sb.endpoints {
		if len(e.Gateway()) > 0 {
			gwepAfter = e
			break
		}
	}
	delete(sb.epPriority, ep.ID())
	sb.mu.Unlock()

	if gwepAfter != nil && gwepBefore != gwepAfter {
		if err := sb.updateGateway(gwepAfter); err != nil {
			return err
		}
	}

	// Only update the store if we did not come here as part of
	// sandbox delete. If we came here as part of delete then do
	// not bother updating the store. The sandbox object will be
	// deleted anyway
	if !inDelete {
		return sb.storeUpdate()
	}

	return nil
}

// joinLeaveStart waits to ensure there are no joins or leaves in progress and
// marks this join/leave in progress without race
func (sb *Sandbox) joinLeaveStart() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	for sb.joinLeaveDone != nil {
		joinLeaveDone := sb.joinLeaveDone
		sb.mu.Unlock()

		<-joinLeaveDone

		sb.mu.Lock()
	}

	sb.joinLeaveDone = make(chan struct{})
}

// joinLeaveEnd marks the end of this join/leave operation and
// signals the same without race to other join and leave waiters
func (sb *Sandbox) joinLeaveEnd() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.joinLeaveDone != nil {
		close(sb.joinLeaveDone)
		sb.joinLeaveDone = nil
	}
}

// OptionHostname function returns an option setter for hostname option to
// be passed to NewSandbox method.
func OptionHostname(name string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.hostName = name
	}
}

// OptionDomainname function returns an option setter for domainname option to
// be passed to NewSandbox method.
func OptionDomainname(name string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.domainName = name
	}
}

// OptionHostsPath function returns an option setter for hostspath option to
// be passed to NewSandbox method.
func OptionHostsPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.hostsPath = path
	}
}

// OptionOriginHostsPath function returns an option setter for origin hosts file path
// to be passed to NewSandbox method.
func OptionOriginHostsPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.originHostsPath = path
	}
}

// OptionExtraHost function returns an option setter for extra /etc/hosts options
// which is a name and IP as strings.
func OptionExtraHost(name string, IP string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.extraHosts = append(sb.config.extraHosts, extraHost{name: name, IP: IP})
	}
}

// OptionParentUpdate function returns an option setter for parent container
// which needs to update the IP address for the linked container.
func OptionParentUpdate(cid string, name, ip string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.parentUpdates = append(sb.config.parentUpdates, parentUpdate{cid: cid, name: name, ip: ip})
	}
}

// OptionResolvConfPath function returns an option setter for resolvconfpath option to
// be passed to net container methods.
func OptionResolvConfPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.resolvConfPath = path
	}
}

// OptionOriginResolvConfPath function returns an option setter to set the path to the
// origin resolv.conf file to be passed to net container methods.
func OptionOriginResolvConfPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.originResolvConfPath = path
	}
}

// OptionDNS function returns an option setter for dns entry option to
// be passed to container Create method.
func OptionDNS(dns string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.dnsList = append(sb.config.dnsList, dns)
	}
}

// OptionDNSSearch function returns an option setter for dns search entry option to
// be passed to container Create method.
func OptionDNSSearch(search string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.dnsSearchList = append(sb.config.dnsSearchList, search)
	}
}

// OptionDNSOptions function returns an option setter for dns options entry option to
// be passed to container Create method.
func OptionDNSOptions(options string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.dnsOptionsList = append(sb.config.dnsOptionsList, options)
	}
}

// OptionUseDefaultSandbox function returns an option setter for using default sandbox
// (host namespace) to be passed to container Create method.
func OptionUseDefaultSandbox() SandboxOption {
	return func(sb *Sandbox) {
		sb.config.useDefaultSandBox = true
	}
}

// OptionUseExternalKey function returns an option setter for using provided namespace
// instead of creating one.
func OptionUseExternalKey() SandboxOption {
	return func(sb *Sandbox) {
		sb.config.useExternalKey = true
	}
}

// OptionGeneric function returns an option setter for Generic configuration
// that is not managed by libNetwork but can be used by the Drivers during the call to
// net container creation method. Container Labels are a good example.
func OptionGeneric(generic map[string]interface{}) SandboxOption {
	return func(sb *Sandbox) {
		if sb.config.generic == nil {
			sb.config.generic = make(map[string]interface{}, len(generic))
		}
		for k, v := range generic {
			sb.config.generic[k] = v
		}
	}
}

// OptionExposedPorts function returns an option setter for the container exposed
// ports option to be passed to container Create method.
func OptionExposedPorts(exposedPorts []types.TransportPort) SandboxOption {
	return func(sb *Sandbox) {
		if sb.config.generic == nil {
			sb.config.generic = make(map[string]interface{})
		}
		// Defensive copy
		eps := make([]types.TransportPort, len(exposedPorts))
		copy(eps, exposedPorts)
		// Store endpoint label and in generic because driver needs it
		sb.config.exposedPorts = eps
		sb.config.generic[netlabel.ExposedPorts] = eps
	}
}

// OptionPortMapping function returns an option setter for the mapping
// ports option to be passed to container Create method.
func OptionPortMapping(portBindings []types.PortBinding) SandboxOption {
	return func(sb *Sandbox) {
		if sb.config.generic == nil {
			sb.config.generic = make(map[string]interface{})
		}
		// Store a copy of the bindings as generic data to pass to the driver
		pbs := make([]types.PortBinding, len(portBindings))
		copy(pbs, portBindings)
		sb.config.generic[netlabel.PortMap] = pbs
	}
}

// OptionIngress function returns an option setter for marking a
// sandbox as the controller's ingress sandbox.
func OptionIngress() SandboxOption {
	return func(sb *Sandbox) {
		sb.ingress = true
		sb.oslTypes = append(sb.oslTypes, osl.SandboxTypeIngress)
	}
}

// OptionLoadBalancer function returns an option setter for marking a
// sandbox as a load balancer sandbox.
func OptionLoadBalancer(nid string) SandboxOption {
	return func(sb *Sandbox) {
		sb.loadBalancerNID = nid
		sb.oslTypes = append(sb.oslTypes, osl.SandboxTypeLoadBalancer)
	}
}

// <=> Returns true if a < b, false if a > b and advances to next level if a == b
// epi.prio <=> epj.prio           # 2 < 1
// epi.gw <=> epj.gw               # non-gw < gw
// epi.internal <=> epj.internal   # non-internal < internal
// epi.joininfo <=> epj.joininfo   # ipv6 < ipv4
// epi.name <=> epj.name           # bar < foo
func (epi *Endpoint) Less(epj *Endpoint) bool {
	var (
		prioi, prioj int
	)

	sbi, _ := epi.getSandbox()
	sbj, _ := epj.getSandbox()

	// Prio defaults to 0
	if sbi != nil {
		prioi = sbi.epPriority[epi.ID()]
	}
	if sbj != nil {
		prioj = sbj.epPriority[epj.ID()]
	}

	if prioi != prioj {
		return prioi > prioj
	}

	gwi := epi.endpointInGWNetwork()
	gwj := epj.endpointInGWNetwork()
	if gwi != gwj {
		return gwj
	}

	inti := epi.getNetwork().Internal()
	intj := epj.getNetwork().Internal()
	if inti != intj {
		return intj
	}

	jii := 0
	if epi.joinInfo != nil {
		if epi.joinInfo.gw != nil {
			jii = jii + 1
		}
		if epi.joinInfo.gw6 != nil {
			jii = jii + 2
		}
	}

	jij := 0
	if epj.joinInfo != nil {
		if epj.joinInfo.gw != nil {
			jij = jij + 1
		}
		if epj.joinInfo.gw6 != nil {
			jij = jij + 2
		}
	}

	if jii != jij {
		return jii > jij
	}

	return epi.network.Name() < epj.network.Name()
}

func (sb *Sandbox) NdotsSet() bool {
	return sb.ndotsSet
}
