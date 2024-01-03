package libnetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/etchosts"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	osSbox             *osl.Namespace
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
			if !c.isSwarmNode() {
				retain = true
			}
			log.G(context.TODO()).Warnf("Failed getting network for ep %s during sandbox %s delete: %v", ep.ID(), sb.ID(), err)
			continue
		}

		if !force {
			if err := ep.Leave(sb); err != nil {
				log.G(context.TODO()).Warnf("Failed detaching sandbox %s from endpoint %s: %v\n", sb.ID(), ep.ID(), err)
			}
		}

		if err := ep.Delete(force); err != nil {
			log.G(context.TODO()).Warnf("Failed deleting endpoint %s: %v\n", ep.ID(), err)
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
			log.G(context.TODO()).WithError(err).Warn("error destroying network sandbox")
		}
	}

	if err := sb.storeDelete(); err != nil {
		log.G(context.TODO()).Warnf("Failed to delete sandbox %s from store: %v", sb.ID(), err)
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
					log.G(context.TODO()).WithField("old", oldName).WithField("origError", err).WithError(err2).Error("error renaming sandbox")
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
			log.G(context.TODO()).Warnf("Failed detaching sandbox %s from endpoint %s: %v\n", sb.ID(), ep.ID(), err)
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
			log.G(context.TODO()).Warnf("Failed attach sandbox %s to endpoint %s: %v\n", sb.ID(), ep.ID(), err)
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

func (sb *Sandbox) HandleQueryResp(name string, ip net.IP) {
	for _, ep := range sb.Endpoints() {
		n := ep.getNetwork()
		n.HandleQueryResp(name, ip)
	}
}

func (sb *Sandbox) ResolveIP(ctx context.Context, ip string) string {
	var svc string
	log.G(ctx).Debugf("IP To resolve %v", ip)

	for _, ep := range sb.Endpoints() {
		n := ep.getNetwork()
		svc = n.ResolveIP(ctx, ip)
		if len(svc) != 0 {
			return svc
		}
	}

	return svc
}

// ResolveService returns all the backend details about the containers or hosts
// backing a service. Its purpose is to satisfy an SRV query.
func (sb *Sandbox) ResolveService(ctx context.Context, name string) ([]*net.SRV, []net.IP) {
	log.G(ctx).Debugf("Service name To resolve: %v", name)

	// There are DNS implementations that allow SRV queries for names not in
	// the format defined by RFC 2782. Hence specific validations checks are
	// not done
	if parts := strings.SplitN(name, ".", 3); len(parts) < 3 {
		return nil, nil
	}

	for _, ep := range sb.Endpoints() {
		n := ep.getNetwork()

		srv, ip := n.ResolveService(ctx, name)
		if len(srv) > 0 {
			return srv, ip
		}
	}
	return nil, nil
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

func (sb *Sandbox) ResolveName(ctx context.Context, name string, ipType int) ([]net.IP, bool) {
	// Embedded server owns the docker network domain. Resolution should work
	// for both container_name and container_name.network_name
	// We allow '.' in service name and network name. For a name a.b.c.d the
	// following have to tried;
	// {a.b.c.d in the networks container is connected to}
	// {a.b.c in network d},
	// {a.b in network c.d},
	// {a in network b.c.d},

	log.G(ctx).Debugf("Name To resolve: %v", name)
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
	if sb.controller.isSwarmNode() {
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
		ip, ipv6Miss := sb.resolveName(ctx, reqName[i], networkName[i], epList, true, ipType)
		if ip != nil {
			return ip, false
		}
		if ipv6Miss {
			return ip, ipv6Miss
		}

		// Resolve the actual container name
		ip, ipv6Miss = sb.resolveName(ctx, reqName[i], networkName[i], epList, false, ipType)
		if ip != nil {
			return ip, false
		}
		if ipv6Miss {
			return ip, ipv6Miss
		}
	}
	return nil, false
}

func (sb *Sandbox) resolveName(ctx context.Context, nameOrAlias string, networkName string, epList []*Endpoint, lookupAlias bool, ipType int) (_ []net.IP, ipv6Miss bool) {
	ctx, span := otel.Tracer("").Start(ctx, "Sandbox.resolveName", trace.WithAttributes(
		attribute.String("libnet.resolver.name-or-alias", nameOrAlias),
		attribute.String("libnet.network.name", networkName),
		attribute.Bool("libnet.resolver.alias-lookup", lookupAlias),
		attribute.Int("libnet.resolver.ip-family", ipType)))
	defer span.End()

	for _, ep := range epList {
		if lookupAlias && len(ep.aliases) == 0 {
			continue
		}

		nw := ep.getNetwork()
		if networkName != "" && networkName != nw.Name() {
			continue
		}

		name := nameOrAlias
		if lookupAlias {
			ep.mu.Lock()
			alias, ok := ep.aliases[nameOrAlias]
			ep.mu.Unlock()
			if !ok {
				continue
			}
			name = alias
		} else {
			// If it is a regular lookup and if the requested name is an alias
			// don't perform a svc lookup for this endpoint.
			ep.mu.Lock()
			_, ok := ep.aliases[nameOrAlias]
			ep.mu.Unlock()
			if ok {
				continue
			}
		}

		ip, miss := nw.ResolveName(ctx, name, ipType)
		if ip != nil {
			return ip, false
		}
		if miss {
			ipv6Miss = miss
		}
	}
	return nil, ipv6Miss
}

// EnableService makes a managed container's service available by adding the
// endpoint to the service load balancer and service discovery.
func (sb *Sandbox) EnableService() (err error) {
	log.G(context.TODO()).Debugf("EnableService %s START", sb.containerID)
	defer func() {
		if err != nil {
			if err2 := sb.DisableService(); err2 != nil {
				log.G(context.TODO()).WithError(err2).WithField("origError", err).Error("Error while disabling service after original error")
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
	log.G(context.TODO()).Debugf("EnableService %s DONE", sb.containerID)
	return nil
}

// DisableService removes a managed container's endpoints from the load balancer
// and service discovery.
func (sb *Sandbox) DisableService() (err error) {
	log.G(context.TODO()).Debugf("DisableService %s START", sb.containerID)
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
				log.G(context.TODO()).Warnf("failed update state for endpoint %s into cluster: %v", ep.Name(), err)
			}
			ep.disableService()
		}
	}
	log.G(context.TODO()).Debugf("DisableService %s DONE", sb.containerID)
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
		log.G(context.TODO()).Errorf("No endpoints in sandbox while trying to remove endpoint %s", ep.Name())
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
		log.G(context.TODO()).Warnf("Endpoint %s has already been deleted", ep.Name())
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

// <=> Returns true if a < b, false if a > b and advances to next level if a == b
// epi.prio <=> epj.prio           # 2 < 1
// epi.gw <=> epj.gw               # non-gw < gw
// epi.internal <=> epj.internal   # non-internal < internal
// epi.joininfo <=> epj.joininfo   # ipv6 < ipv4
// epi.name <=> epj.name           # bar < foo
func (epi *Endpoint) Less(epj *Endpoint) bool {
	var prioi, prioj int

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
