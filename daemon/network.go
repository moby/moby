package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/container"
	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	"github.com/docker/docker/daemon/config"
	internalnetwork "github.com/docker/docker/daemon/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork"
	lncluster "github.com/docker/docker/libnetwork/cluster"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/networkdb"
	"github.com/docker/docker/libnetwork/options"
	networktypes "github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/runconfig"
	"github.com/docker/go-connections/nat"
)

// PredefinedNetworkError is returned when user tries to create predefined network that already exists.
type PredefinedNetworkError string

func (pnr PredefinedNetworkError) Error() string {
	return fmt.Sprintf("operation is not permitted on predefined %s network ", string(pnr))
}

// Forbidden denotes the type of this error
func (pnr PredefinedNetworkError) Forbidden() {}

// NetworkControllerEnabled checks if the networking stack is enabled.
// This feature depends on OS primitives and it's disabled in systems like Windows.
func (daemon *Daemon) NetworkControllerEnabled() bool {
	return daemon.netController != nil
}

// NetworkController returns the network controller created by the daemon.
func (daemon *Daemon) NetworkController() *libnetwork.Controller {
	return daemon.netController
}

// FindNetwork returns a network based on:
// 1. Full ID
// 2. Full Name
// 3. Partial ID
// as long as there is no ambiguity
func (daemon *Daemon) FindNetwork(term string) (*libnetwork.Network, error) {
	var listByFullName, listByPartialID []*libnetwork.Network
	for _, nw := range daemon.getAllNetworks() {
		nwID := nw.ID()
		if nwID == term {
			return nw, nil
		}
		if strings.HasPrefix(nw.ID(), term) {
			listByPartialID = append(listByPartialID, nw)
		}
		if nw.Name() == term {
			listByFullName = append(listByFullName, nw)
		}
	}
	switch {
	case len(listByFullName) == 1:
		return listByFullName[0], nil
	case len(listByFullName) > 1:
		return nil, errdefs.InvalidParameter(fmt.Errorf("network %s is ambiguous (%d matches found on name)", term, len(listByFullName)))
	case len(listByPartialID) == 1:
		return listByPartialID[0], nil
	case len(listByPartialID) > 1:
		return nil, errdefs.InvalidParameter(fmt.Errorf("network %s is ambiguous (%d matches found based on ID prefix)", term, len(listByPartialID)))
	}

	// Be very careful to change the error type here, the
	// libnetwork.ErrNoSuchNetwork error is used by the controller
	// to retry the creation of the network as managed through the swarm manager
	return nil, errdefs.NotFound(libnetwork.ErrNoSuchNetwork(term))
}

// GetNetworkByID function returns a network whose ID matches the given ID.
// It fails with an error if no matching network is found.
func (daemon *Daemon) GetNetworkByID(id string) (*libnetwork.Network, error) {
	c := daemon.netController
	if c == nil {
		return nil, fmt.Errorf("netcontroller is nil: %w", libnetwork.ErrNoSuchNetwork(id))
	}
	return c.NetworkByID(id)
}

// GetNetworkByName function returns a network for a given network name.
// If no network name is given, the default network is returned.
func (daemon *Daemon) GetNetworkByName(name string) (*libnetwork.Network, error) {
	c := daemon.netController
	if c == nil {
		return nil, libnetwork.ErrNoSuchNetwork(name)
	}
	if name == "" {
		name = c.Config().DefaultNetwork
	}
	return c.NetworkByName(name)
}

// GetNetworksByIDPrefix returns a list of networks whose ID partially matches zero or more networks
func (daemon *Daemon) GetNetworksByIDPrefix(partialID string) []*libnetwork.Network {
	c := daemon.netController
	if c == nil {
		return nil
	}
	list := []*libnetwork.Network{}
	l := func(nw *libnetwork.Network) bool {
		if strings.HasPrefix(nw.ID(), partialID) {
			list = append(list, nw)
		}
		return false
	}
	c.WalkNetworks(l)

	return list
}

// getAllNetworks returns a list containing all networks
func (daemon *Daemon) getAllNetworks() []*libnetwork.Network {
	c := daemon.netController
	if c == nil {
		return nil
	}
	ctx := context.TODO()
	return c.Networks(ctx)
}

type ingressJob struct {
	create  *clustertypes.NetworkCreateRequest
	ip      net.IP
	jobDone chan struct{}
}

var (
	ingressWorkerOnce  sync.Once
	ingressJobsChannel chan *ingressJob
	ingressID          string
)

func (daemon *Daemon) startIngressWorker() {
	ingressJobsChannel = make(chan *ingressJob, 100)
	go func() {
		//nolint: gosimple
		for {
			select {
			case r := <-ingressJobsChannel:
				if r.create != nil {
					daemon.setupIngress(&daemon.config().Config, r.create, r.ip, ingressID)
					ingressID = r.create.ID
				} else {
					daemon.releaseIngress(ingressID)
					ingressID = ""
				}
				close(r.jobDone)
			}
		}
	}()
}

// enqueueIngressJob adds a ingress add/rm request to the worker queue.
// It guarantees the worker is started.
func (daemon *Daemon) enqueueIngressJob(job *ingressJob) {
	ingressWorkerOnce.Do(daemon.startIngressWorker)
	ingressJobsChannel <- job
}

// SetupIngress setups ingress networking.
// The function returns a channel which will signal the caller when the programming is completed.
func (daemon *Daemon) SetupIngress(create clustertypes.NetworkCreateRequest, nodeIP string) (<-chan struct{}, error) {
	ip, _, err := net.ParseCIDR(nodeIP)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	daemon.enqueueIngressJob(&ingressJob{&create, ip, done})
	return done, nil
}

// ReleaseIngress releases the ingress networking.
// The function returns a channel which will signal the caller when the programming is completed.
func (daemon *Daemon) ReleaseIngress() (<-chan struct{}, error) {
	done := make(chan struct{})
	daemon.enqueueIngressJob(&ingressJob{nil, nil, done})
	return done, nil
}

func (daemon *Daemon) setupIngress(cfg *config.Config, create *clustertypes.NetworkCreateRequest, ip net.IP, staleID string) {
	controller := daemon.netController
	controller.AgentInitWait()

	if staleID != "" && staleID != create.ID {
		daemon.releaseIngress(staleID)
	}

	if _, err := daemon.createNetwork(cfg, create.CreateRequest, create.ID, true); err != nil {
		// If it is any other error other than already
		// exists error log error and return.
		if _, ok := err.(libnetwork.NetworkNameError); !ok {
			log.G(context.TODO()).Errorf("Failed creating ingress network: %v", err)
			return
		}
		// Otherwise continue down the call to create or recreate sandbox.
	}

	_, err := daemon.GetNetworkByID(create.ID)
	if err != nil {
		log.G(context.TODO()).Errorf("Failed getting ingress network by id after creating: %v", err)
	}
}

func (daemon *Daemon) releaseIngress(id string) {
	controller := daemon.netController

	if id == "" {
		return
	}

	n, err := controller.NetworkByID(id)
	if err != nil {
		log.G(context.TODO()).Errorf("failed to retrieve ingress network %s: %v", id, err)
		return
	}

	if err := n.Delete(libnetwork.NetworkDeleteOptionRemoveLB); err != nil {
		log.G(context.TODO()).Errorf("Failed to delete ingress network %s: %v", n.ID(), err)
		return
	}
}

// SetNetworkBootstrapKeys sets the bootstrap keys.
func (daemon *Daemon) SetNetworkBootstrapKeys(keys []*networktypes.EncryptionKey) error {
	if err := daemon.netController.SetKeys(keys); err != nil {
		return err
	}
	// Upon successful key setting dispatch the keys available event
	daemon.cluster.SendClusterEvent(lncluster.EventNetworkKeysAvailable)
	return nil
}

// UpdateAttachment notifies the attacher about the attachment config.
func (daemon *Daemon) UpdateAttachment(networkName, networkID, containerID string, config *network.NetworkingConfig) error {
	if daemon.clusterProvider == nil {
		return fmt.Errorf("cluster provider is not initialized")
	}

	if err := daemon.clusterProvider.UpdateAttachment(networkName, containerID, config); err != nil {
		return daemon.clusterProvider.UpdateAttachment(networkID, containerID, config)
	}

	return nil
}

// WaitForDetachment makes the cluster manager wait for detachment of
// the container from the network.
func (daemon *Daemon) WaitForDetachment(ctx context.Context, networkName, networkID, taskID, containerID string) error {
	if daemon.clusterProvider == nil {
		return fmt.Errorf("cluster provider is not initialized")
	}

	return daemon.clusterProvider.WaitForDetachment(ctx, networkName, networkID, taskID, containerID)
}

// CreateManagedNetwork creates an agent network.
func (daemon *Daemon) CreateManagedNetwork(create clustertypes.NetworkCreateRequest) error {
	_, err := daemon.createNetwork(&daemon.config().Config, create.CreateRequest, create.ID, true)
	return err
}

// CreateNetwork creates a network with the given name, driver and other optional parameters
func (daemon *Daemon) CreateNetwork(create network.CreateRequest) (*network.CreateResponse, error) {
	return daemon.createNetwork(&daemon.config().Config, create, "", false)
}

func (daemon *Daemon) createNetwork(cfg *config.Config, create network.CreateRequest, id string, agent bool) (*network.CreateResponse, error) {
	if runconfig.IsPreDefinedNetwork(create.Name) {
		return nil, PredefinedNetworkError(create.Name)
	}

	c := daemon.netController
	driver := create.Driver
	if driver == "" {
		driver = c.Config().DefaultDriver
	}

	if driver == "overlay" && !daemon.cluster.IsManager() && !agent {
		return nil, errdefs.Forbidden(errors.New(`This node is not a swarm manager. Use "docker swarm init" or "docker swarm join" to connect this node to swarm and try again.`))
	}

	networkOptions := make(map[string]string)
	for k, v := range create.Options {
		networkOptions[k] = v
	}
	if defaultOpts, ok := cfg.DefaultNetworkOpts[driver]; create.ConfigFrom == nil && ok {
		for k, v := range defaultOpts {
			if _, ok := networkOptions[k]; !ok {
				log.G(context.TODO()).WithFields(log.Fields{"driver": driver, "network": id, k: v}).Debug("Applying network default option")
				networkOptions[k] = v
			}
		}
	}

	var enableIPv6 bool
	if create.EnableIPv6 != nil {
		enableIPv6 = *create.EnableIPv6
	} else {
		var err error
		v, ok := networkOptions[netlabel.EnableIPv6]
		if enableIPv6, err = strconv.ParseBool(v); ok && err != nil {
			return nil, errdefs.InvalidParameter(fmt.Errorf("driver-opt %q is not a valid bool", netlabel.EnableIPv6))
		}
	}

	nwOptions := []libnetwork.NetworkOption{
		libnetwork.NetworkOptionEnableIPv6(enableIPv6),
		libnetwork.NetworkOptionDriverOpts(networkOptions),
		libnetwork.NetworkOptionLabels(create.Labels),
		libnetwork.NetworkOptionAttachable(create.Attachable),
		libnetwork.NetworkOptionIngress(create.Ingress),
		libnetwork.NetworkOptionScope(create.Scope),
	}

	if create.ConfigOnly {
		nwOptions = append(nwOptions, libnetwork.NetworkOptionConfigOnly())
	}

	if err := network.ValidateIPAM(create.IPAM, enableIPv6); err != nil {
		if agent {
			// This function is called with agent=false for all networks. For swarm-scoped
			// networks, the configuration is validated but ManagerRedirectError is returned
			// and the network is not created. Then, each time a swarm-scoped network is
			// needed, this function is called again with agent=true.
			//
			// Non-swarm networks created before ValidateIPAM was introduced continue to work
			// as they did before-upgrade, even if they would fail the new checks on creation
			// (for example, by having host-bits set in their subnet). Those networks are not
			// seen again here.
			//
			// By dropping errors for agent networks, existing swarm-scoped networks also
			// continue to behave as they did before upgrade - but new networks are still
			// validated.
			log.G(context.TODO()).WithFields(log.Fields{
				"error":   err,
				"network": create.Name,
			}).Warn("Continuing with validation errors in agent IPAM")
		} else {
			return nil, errdefs.InvalidParameter(err)
		}
	}

	if create.IPAM != nil {
		ipam := create.IPAM
		v4Conf, v6Conf, err := getIpamConfig(ipam.Config)
		if err != nil {
			return nil, err
		}
		nwOptions = append(nwOptions, libnetwork.NetworkOptionIpam(ipam.Driver, "", v4Conf, v6Conf, ipam.Options))
	}

	if create.Internal {
		nwOptions = append(nwOptions, libnetwork.NetworkOptionInternalNetwork())
	}
	if agent {
		nwOptions = append(nwOptions, libnetwork.NetworkOptionDynamic())
		nwOptions = append(nwOptions, libnetwork.NetworkOptionPersist(false))
	}

	if create.ConfigFrom != nil {
		nwOptions = append(nwOptions, libnetwork.NetworkOptionConfigFrom(create.ConfigFrom.Network))
	}

	if agent && driver == "overlay" {
		nodeIP, exists := daemon.GetAttachmentStore().GetIPForNetwork(id)
		if !exists {
			return nil, fmt.Errorf("failed to find a load balancer IP to use for network: %v", id)
		}

		nwOptions = append(nwOptions, libnetwork.NetworkOptionLBEndpoint(nodeIP))
	}

	n, err := c.NewNetwork(driver, create.Name, id, nwOptions...)
	if err != nil {
		return nil, err
	}

	daemon.pluginRefCount(driver, driverapi.NetworkPluginEndpointType, plugingetter.Acquire)
	if create.IPAM != nil {
		daemon.pluginRefCount(create.IPAM.Driver, ipamapi.PluginEndpointType, plugingetter.Acquire)
	}
	daemon.LogNetworkEvent(n, events.ActionCreate)

	return &network.CreateResponse{ID: n.ID()}, nil
}

func (daemon *Daemon) pluginRefCount(driver, capability string, mode int) {
	var builtinDrivers []string

	if capability == driverapi.NetworkPluginEndpointType {
		builtinDrivers = daemon.netController.BuiltinDrivers()
	} else if capability == ipamapi.PluginEndpointType {
		builtinDrivers = daemon.netController.BuiltinIPAMDrivers()
	}

	for _, d := range builtinDrivers {
		if d == driver {
			return
		}
	}

	if daemon.PluginStore != nil {
		_, err := daemon.PluginStore.Get(driver, capability, mode)
		if err != nil {
			log.G(context.TODO()).WithError(err).WithFields(log.Fields{"mode": mode, "driver": driver}).Error("Error handling plugin refcount operation")
		}
	}
}

func getIpamConfig(data []network.IPAMConfig) ([]*libnetwork.IpamConf, []*libnetwork.IpamConf, error) {
	ipamV4Cfg := []*libnetwork.IpamConf{}
	ipamV6Cfg := []*libnetwork.IpamConf{}
	for _, d := range data {
		iCfg := libnetwork.IpamConf{}
		iCfg.PreferredPool = d.Subnet
		iCfg.SubPool = d.IPRange
		iCfg.Gateway = d.Gateway
		iCfg.AuxAddresses = d.AuxAddress
		ip, _, err := net.ParseCIDR(d.Subnet)
		if err != nil {
			return nil, nil, fmt.Errorf("Invalid subnet %s : %v", d.Subnet, err)
		}
		if ip.To4() != nil {
			ipamV4Cfg = append(ipamV4Cfg, &iCfg)
		} else {
			ipamV6Cfg = append(ipamV6Cfg, &iCfg)
		}
	}
	return ipamV4Cfg, ipamV6Cfg, nil
}

// UpdateContainerServiceConfig updates a service configuration.
func (daemon *Daemon) UpdateContainerServiceConfig(containerName string, serviceConfig *clustertypes.ServiceConfig) error {
	ctr, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}

	ctr.NetworkSettings.Service = serviceConfig
	return nil
}

// ConnectContainerToNetwork connects the given container to the given
// network. If either cannot be found, an err is returned. If the
// network cannot be set up, an err is returned.
func (daemon *Daemon) ConnectContainerToNetwork(ctx context.Context, containerName, networkName string, endpointConfig *network.EndpointSettings) error {
	ctr, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}
	return daemon.ConnectToNetwork(ctx, ctr, networkName, endpointConfig)
}

// DisconnectContainerFromNetwork disconnects the given container from
// the given network. If either cannot be found, an err is returned.
func (daemon *Daemon) DisconnectContainerFromNetwork(containerName string, networkName string, force bool) error {
	ctr, err := daemon.GetContainer(containerName)
	if err != nil {
		if force {
			return daemon.ForceEndpointDelete(containerName, networkName)
		}
		return err
	}
	return daemon.DisconnectFromNetwork(context.TODO(), ctr, networkName, force)
}

// GetNetworkDriverList returns the list of plugins drivers
// registered for network.
func (daemon *Daemon) GetNetworkDriverList(ctx context.Context) []string {
	if !daemon.NetworkControllerEnabled() {
		return nil
	}

	pluginList := daemon.netController.BuiltinDrivers()

	managedPlugins := daemon.PluginStore.GetAllManagedPluginsByCap(driverapi.NetworkPluginEndpointType)

	for _, plugin := range managedPlugins {
		pluginList = append(pluginList, plugin.Name())
	}

	pluginMap := make(map[string]bool)
	for _, plugin := range pluginList {
		pluginMap[plugin] = true
	}

	networks := daemon.netController.Networks(ctx)

	for _, nw := range networks {
		if !pluginMap[nw.Type()] {
			pluginList = append(pluginList, nw.Type())
			pluginMap[nw.Type()] = true
		}
	}

	sort.Strings(pluginList)

	return pluginList
}

// DeleteManagedNetwork deletes an agent network.
// The requirement of networkID is enforced.
func (daemon *Daemon) DeleteManagedNetwork(networkID string) error {
	n, err := daemon.GetNetworkByID(networkID)
	if err != nil {
		return err
	}
	return daemon.deleteNetwork(n, true)
}

// DeleteNetwork destroys a network unless it's one of docker's predefined networks.
func (daemon *Daemon) DeleteNetwork(networkID string) error {
	n, err := daemon.GetNetworkByID(networkID)
	if err != nil {
		return fmt.Errorf("could not find network by ID: %w", err)
	}
	return daemon.deleteNetwork(n, false)
}

func (daemon *Daemon) deleteNetwork(nw *libnetwork.Network, dynamic bool) error {
	if runconfig.IsPreDefinedNetwork(nw.Name()) && !dynamic {
		err := fmt.Errorf("%s is a pre-defined network and cannot be removed", nw.Name())
		return errdefs.Forbidden(err)
	}

	if dynamic && !nw.Dynamic() {
		if runconfig.IsPreDefinedNetwork(nw.Name()) {
			// Predefined networks now support swarm services. Make this
			// a no-op when cluster requests to remove the predefined network.
			return nil
		}
		err := fmt.Errorf("%s is not a dynamic network", nw.Name())
		return errdefs.Forbidden(err)
	}

	if err := nw.Delete(); err != nil {
		return fmt.Errorf("error while removing network: %w", err)
	}

	// If this is not a configuration only network, we need to
	// update the corresponding remote drivers' reference counts
	if !nw.ConfigOnly() {
		daemon.pluginRefCount(nw.Type(), driverapi.NetworkPluginEndpointType, plugingetter.Release)
		ipamType, _, _, _ := nw.IpamConfig()
		daemon.pluginRefCount(ipamType, ipamapi.PluginEndpointType, plugingetter.Release)
		daemon.LogNetworkEvent(nw, events.ActionDestroy)
	}

	return nil
}

// GetNetworks returns a list of all networks
func (daemon *Daemon) GetNetworks(filter filters.Args, config backend.NetworkListConfig) (networks []network.Inspect, err error) {
	var idx map[string]*libnetwork.Network
	if config.Detailed {
		idx = make(map[string]*libnetwork.Network)
	}

	allNetworks := daemon.getAllNetworks()
	networks = make([]network.Inspect, 0, len(allNetworks))
	for _, n := range allNetworks {
		nr := buildNetworkResource(n)
		networks = append(networks, nr)
		if config.Detailed {
			idx[nr.ID] = n
		}
	}

	networks, err = internalnetwork.FilterNetworks(networks, filter)
	if err != nil {
		return nil, err
	}

	if config.Detailed {
		for i, nw := range networks {
			networks[i].Containers = buildContainerAttachments(idx[nw.ID])
			if config.Verbose {
				networks[i].Services = buildServiceAttachments(idx[nw.ID])
			}
		}
	}

	return networks, nil
}

// buildNetworkResource builds a [types.NetworkResource] from the given
// [libnetwork.Network], to be returned by the API.
func buildNetworkResource(nw *libnetwork.Network) network.Inspect {
	if nw == nil {
		return network.Inspect{}
	}

	return network.Inspect{
		Name:       nw.Name(),
		ID:         nw.ID(),
		Created:    nw.Created(),
		Scope:      nw.Scope(),
		Driver:     nw.Type(),
		EnableIPv6: nw.IPv6Enabled(),
		IPAM:       buildIPAMResources(nw),
		Internal:   nw.Internal(),
		Attachable: nw.Attachable(),
		Ingress:    nw.Ingress(),
		ConfigFrom: network.ConfigReference{Network: nw.ConfigFrom()},
		ConfigOnly: nw.ConfigOnly(),
		Containers: map[string]network.EndpointResource{},
		Options:    nw.DriverOptions(),
		Labels:     nw.Labels(),
		Peers:      buildPeerInfoResources(nw.Peers()),
	}
}

// buildContainerAttachments creates a [types.EndpointResource] map of all
// containers attached to the network. It is used when listing networks in
// detailed mode.
func buildContainerAttachments(nw *libnetwork.Network) map[string]network.EndpointResource {
	containers := make(map[string]network.EndpointResource)
	for _, e := range nw.Endpoints() {
		ei := e.Info()
		if ei == nil {
			continue
		}
		if sb := ei.Sandbox(); sb != nil {
			containers[sb.ContainerID()] = buildEndpointResource(e, ei)
		} else {
			containers["ep-"+e.ID()] = buildEndpointResource(e, ei)
		}
	}
	return containers
}

// buildServiceAttachments creates a [network.ServiceInfo] map of all services
// attached to the network. It is used when listing networks in "verbose" mode.
func buildServiceAttachments(nw *libnetwork.Network) map[string]network.ServiceInfo {
	services := make(map[string]network.ServiceInfo)
	for name, service := range nw.Services() {
		tasks := make([]network.Task, 0, len(service.Tasks))
		for _, t := range service.Tasks {
			tasks = append(tasks, network.Task{
				Name:       t.Name,
				EndpointID: t.EndpointID,
				EndpointIP: t.EndpointIP,
				Info:       t.Info,
			})
		}
		services[name] = network.ServiceInfo{
			VIP:          service.VIP,
			Ports:        service.Ports,
			Tasks:        tasks,
			LocalLBIndex: service.LocalLBIndex,
		}
	}
	return services
}

// buildPeerInfoResources converts a list of [networkdb.PeerInfo] to a
// [network.PeerInfo] for inclusion in API responses. It returns nil if
// the list of peers is empty.
func buildPeerInfoResources(peers []networkdb.PeerInfo) []network.PeerInfo {
	if len(peers) == 0 {
		return nil
	}
	peerInfo := make([]network.PeerInfo, 0, len(peers))
	for _, peer := range peers {
		peerInfo = append(peerInfo, network.PeerInfo(peer))
	}
	return peerInfo
}

// buildIPAMResources constructs a [network.IPAM] from the network's
// IPAM information for inclusion in API responses.
func buildIPAMResources(nw *libnetwork.Network) network.IPAM {
	var ipamConfig []network.IPAMConfig

	ipamDriver, ipamOptions, ipv4Conf, ipv6Conf := nw.IpamConfig()

	hasIPv4Config := false
	for _, cfg := range ipv4Conf {
		if cfg.PreferredPool == "" {
			continue
		}
		hasIPv4Config = true
		ipamConfig = append(ipamConfig, network.IPAMConfig{
			Subnet:     cfg.PreferredPool,
			IPRange:    cfg.SubPool,
			Gateway:    cfg.Gateway,
			AuxAddress: cfg.AuxAddresses,
		})
	}

	hasIPv6Config := false
	for _, cfg := range ipv6Conf {
		if cfg.PreferredPool == "" {
			continue
		}
		hasIPv6Config = true
		ipamConfig = append(ipamConfig, network.IPAMConfig{
			Subnet:     cfg.PreferredPool,
			IPRange:    cfg.SubPool,
			Gateway:    cfg.Gateway,
			AuxAddress: cfg.AuxAddresses,
		})
	}

	if !hasIPv4Config || !hasIPv6Config {
		ipv4Info, ipv6Info := nw.IpamInfo()
		if !hasIPv4Config {
			for _, info := range ipv4Info {
				var gw string
				if info.IPAMData.Gateway != nil {
					gw = info.IPAMData.Gateway.IP.String()
				}
				ipamConfig = append(ipamConfig, network.IPAMConfig{
					Subnet:  info.IPAMData.Pool.String(),
					Gateway: gw,
				})
			}
		}

		if !hasIPv6Config {
			for _, info := range ipv6Info {
				if info.IPAMData.Pool == nil {
					continue
				}
				ipamConfig = append(ipamConfig, network.IPAMConfig{
					Subnet:  info.IPAMData.Pool.String(),
					Gateway: info.IPAMData.Gateway.String(),
				})
			}
		}
	}

	return network.IPAM{
		Driver:  ipamDriver,
		Options: ipamOptions,
		Config:  ipamConfig,
	}
}

// buildEndpointResource combines information from the endpoint and additional
// endpoint-info into a [types.EndpointResource].
func buildEndpointResource(ep *libnetwork.Endpoint, info libnetwork.EndpointInfo) network.EndpointResource {
	er := network.EndpointResource{
		EndpointID: ep.ID(),
		Name:       ep.Name(),
	}
	if iface := info.Iface(); iface != nil {
		if mac := iface.MacAddress(); mac != nil {
			er.MacAddress = mac.String()
		}
		if ip := iface.Address(); ip != nil && len(ip.IP) > 0 {
			er.IPv4Address = ip.String()
		}
		if ip := iface.AddressIPv6(); ip != nil && len(ip.IP) > 0 {
			er.IPv6Address = ip.String()
		}
	}
	return er
}

// clearAttachableNetworks removes the attachable networks
// after disconnecting any connected container
func (daemon *Daemon) clearAttachableNetworks() {
	for _, n := range daemon.getAllNetworks() {
		if !n.Attachable() {
			continue
		}
		for _, ep := range n.Endpoints() {
			epInfo := ep.Info()
			if epInfo == nil {
				continue
			}
			sb := epInfo.Sandbox()
			if sb == nil {
				continue
			}
			containerID := sb.ContainerID()
			if err := daemon.DisconnectContainerFromNetwork(containerID, n.ID(), true); err != nil {
				log.G(context.TODO()).Warnf("Failed to disconnect container %s from swarm network %s on cluster leave: %v",
					containerID, n.Name(), err)
			}
		}
		if err := daemon.DeleteManagedNetwork(n.ID()); err != nil {
			log.G(context.TODO()).Warnf("Failed to remove swarm network %s on cluster leave: %v", n.Name(), err)
		}
	}
}

// buildCreateEndpointOptions builds endpoint options from a given network.
func buildCreateEndpointOptions(c *container.Container, n *libnetwork.Network, epConfig *internalnetwork.EndpointSettings, sb *libnetwork.Sandbox, daemonDNS []string) ([]libnetwork.EndpointOption, error) {
	var createOptions []libnetwork.EndpointOption
	var genericOptions = make(options.Generic)

	nwName := n.Name()

	if epConfig != nil {
		if ipam := epConfig.IPAMConfig; ipam != nil {
			var ipList []net.IP
			for _, ips := range ipam.LinkLocalIPs {
				linkIP := net.ParseIP(ips)
				if linkIP == nil && ips != "" {
					return nil, fmt.Errorf("invalid link-local IP address: %s", ipam.LinkLocalIPs)
				}
				ipList = append(ipList, linkIP)
			}

			ip := net.ParseIP(ipam.IPv4Address)
			if ip == nil && ipam.IPv4Address != "" {
				return nil, fmt.Errorf("invalid IPv4 address: %s", ipam.IPv4Address)
			}

			ip6 := net.ParseIP(ipam.IPv6Address)
			if ip6 == nil && ipam.IPv6Address != "" {
				return nil, fmt.Errorf("invalid IPv6 address: %s", ipam.IPv6Address)
			}

			createOptions = append(createOptions, libnetwork.CreateOptionIpam(ip, ip6, ipList, nil))
		}

		createOptions = append(createOptions, libnetwork.CreateOptionDNSNames(epConfig.DNSNames))

		for k, v := range epConfig.DriverOpts {
			createOptions = append(createOptions, libnetwork.EndpointOptionGeneric(options.Generic{k: v}))
		}

		if epConfig.DesiredMacAddress != "" {
			mac, err := net.ParseMAC(epConfig.DesiredMacAddress)
			if err != nil {
				return nil, err
			}
			genericOptions[netlabel.MacAddress] = mac
		}
	}

	if svcCfg := c.NetworkSettings.Service; svcCfg != nil {
		nwID := n.ID()

		var vip net.IP
		if virtualAddress := svcCfg.VirtualAddresses[nwID]; virtualAddress != nil {
			vip = net.ParseIP(virtualAddress.IPv4)
		}

		var portConfigs []*libnetwork.PortConfig
		for _, portConfig := range svcCfg.ExposedPorts {
			portConfigs = append(portConfigs, &libnetwork.PortConfig{
				Name:          portConfig.Name,
				Protocol:      libnetwork.PortConfig_Protocol(portConfig.Protocol),
				TargetPort:    portConfig.TargetPort,
				PublishedPort: portConfig.PublishedPort,
			})
		}

		createOptions = append(createOptions, libnetwork.CreateOptionService(svcCfg.Name, svcCfg.ID, vip, portConfigs, svcCfg.Aliases[nwID]))
	}

	// Don't run an internal DNS resolver for host/container/none networks.
	if nm := containertypes.NetworkMode(nwName); nm.IsHost() || nm.IsContainer() || nm.IsNone() {
		createOptions = append(createOptions, libnetwork.CreateOptionDisableResolution())
	}

	opts, err := buildPortsRelatedCreateEndpointOptions(c, n, sb)
	if err != nil {
		return nil, err
	}
	createOptions = append(createOptions, opts...)

	// On Windows, DNS config is a per-adapter config option whereas on Linux, it's a sandbox-wide parameter; hence why
	// we're dealing with DNS config both here and in buildSandboxOptions. Following DNS options are only honored by
	// Windows netdrivers, whereas DNS options in buildSandboxOptions are only honored by Linux netdrivers.
	if !n.Internal() {
		if len(c.HostConfig.DNS) > 0 {
			createOptions = append(createOptions, libnetwork.CreateOptionDNS(c.HostConfig.DNS))
		} else if len(daemonDNS) > 0 {
			createOptions = append(createOptions, libnetwork.CreateOptionDNS(daemonDNS))
		}
	}

	createOptions = append(createOptions, libnetwork.EndpointOptionGeneric(genericOptions))

	return createOptions, nil
}

// buildPortsRelatedCreateEndpointOptions returns the appropriate endpoint options to apply config related to port
// mapping and exposed ports.
func buildPortsRelatedCreateEndpointOptions(c *container.Container, n *libnetwork.Network, sb *libnetwork.Sandbox) ([]libnetwork.EndpointOption, error) {
	// Port-mapping rules belong to the container & applicable only to non-internal networks.
	//
	// TODO(thaJeztah): Look if we can provide a more minimal function for getPortMapInfo, as it does a lot, and we only need the "length".
	if n.Internal() || len(getPortMapInfo(sb)) > 0 {
		return nil, nil
	}

	bindings := make(nat.PortMap)
	if c.HostConfig.PortBindings != nil {
		for p, b := range c.HostConfig.PortBindings {
			bindings[p] = []nat.PortBinding{}
			for _, bb := range b {
				bindings[p] = append(bindings[p], nat.PortBinding{
					HostIP:   bb.HostIP,
					HostPort: bb.HostPort,
				})
			}
		}
	}

	// TODO(thaJeztah): Move this code to a method on nat.PortSet.
	ports := make([]nat.Port, 0, len(c.Config.ExposedPorts))
	for p := range c.Config.ExposedPorts {
		ports = append(ports, p)
	}
	nat.SortPortMap(ports, bindings)

	var (
		exposedPorts   []networktypes.TransportPort
		publishedPorts []networktypes.PortBinding
	)
	for _, port := range ports {
		portProto := networktypes.ParseProtocol(port.Proto())
		portNum := uint16(port.Int())
		exposedPorts = append(exposedPorts, networktypes.TransportPort{
			Proto: portProto,
			Port:  portNum,
		})

		for _, binding := range bindings[port] {
			newP, err := nat.NewPort(nat.SplitProtoPort(binding.HostPort))
			var portStart, portEnd int
			if err == nil {
				portStart, portEnd, err = newP.Range()
			}
			if err != nil {
				return nil, fmt.Errorf("error parsing HostPort value (%s): %w", binding.HostPort, err)
			}
			publishedPorts = append(publishedPorts, networktypes.PortBinding{
				Proto:       portProto,
				Port:        portNum,
				HostIP:      net.ParseIP(binding.HostIP),
				HostPort:    uint16(portStart),
				HostPortEnd: uint16(portEnd),
			})
		}

		if c.HostConfig.PublishAllPorts && len(bindings[port]) == 0 {
			publishedPorts = append(publishedPorts, networktypes.PortBinding{
				Proto: portProto,
				Port:  portNum,
			})
		}
	}

	return []libnetwork.EndpointOption{
		libnetwork.CreateOptionPortMapping(publishedPorts),
		libnetwork.CreateOptionExposedPorts(exposedPorts),
	}, nil
}

// getPortMapInfo retrieves the current port-mapping programmed for the given sandbox
func getPortMapInfo(sb *libnetwork.Sandbox) nat.PortMap {
	pm := nat.PortMap{}
	if sb == nil {
		return pm
	}

	for _, ep := range sb.Endpoints() {
		pm, _ = getEndpointPortMapInfo(ep)
		if len(pm) > 0 {
			break
		}
	}
	return pm
}

func getEndpointPortMapInfo(ep *libnetwork.Endpoint) (nat.PortMap, error) {
	pm := nat.PortMap{}
	driverInfo, err := ep.DriverInfo()
	if err != nil {
		return pm, err
	}

	if driverInfo == nil {
		// It is not an error for epInfo to be nil
		return pm, nil
	}

	if expData, ok := driverInfo[netlabel.ExposedPorts]; ok {
		if exposedPorts, ok := expData.([]networktypes.TransportPort); ok {
			for _, tp := range exposedPorts {
				natPort, err := nat.NewPort(tp.Proto.String(), strconv.Itoa(int(tp.Port)))
				if err != nil {
					return pm, fmt.Errorf("Error parsing Port value(%v):%v", tp.Port, err)
				}
				pm[natPort] = nil
			}
		}
	}

	mapData, ok := driverInfo[netlabel.PortMap]
	if !ok {
		return pm, nil
	}

	if portMapping, ok := mapData.([]networktypes.PortBinding); ok {
		for _, pp := range portMapping {
			// Use an empty string for the host port if there's no port assigned.
			natPort, err := nat.NewPort(pp.Proto.String(), strconv.Itoa(int(pp.Port)))
			if err != nil {
				return pm, err
			}
			var hp string
			if pp.HostPort > 0 {
				hp = strconv.Itoa(int(pp.HostPort))
			}
			natBndg := nat.PortBinding{HostIP: pp.HostIP.String(), HostPort: hp}
			pm[natPort] = append(pm[natPort], natBndg)
		}
	}

	return pm, nil
}

// buildEndpointInfo sets endpoint-related fields on container.NetworkSettings based on the provided network and endpoint.
func buildEndpointInfo(networkSettings *internalnetwork.Settings, n *libnetwork.Network, ep *libnetwork.Endpoint) error {
	if ep == nil {
		return errors.New("endpoint cannot be nil")
	}

	if networkSettings == nil {
		return errors.New("network cannot be nil")
	}

	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return nil
	}

	nwName := n.Name()
	if _, ok := networkSettings.Networks[nwName]; !ok {
		networkSettings.Networks[nwName] = &internalnetwork.EndpointSettings{
			EndpointSettings: &network.EndpointSettings{},
		}
	}
	networkSettings.Networks[nwName].NetworkID = n.ID()
	networkSettings.Networks[nwName].EndpointID = ep.ID()

	iface := epInfo.Iface()
	if iface == nil {
		return nil
	}

	if iface.MacAddress() != nil {
		networkSettings.Networks[nwName].MacAddress = iface.MacAddress().String()
	}

	if iface.Address() != nil {
		ones, _ := iface.Address().Mask.Size()
		networkSettings.Networks[nwName].IPAddress = iface.Address().IP.String()
		networkSettings.Networks[nwName].IPPrefixLen = ones
	}

	if iface.AddressIPv6() != nil && iface.AddressIPv6().IP.To16() != nil {
		onesv6, _ := iface.AddressIPv6().Mask.Size()
		networkSettings.Networks[nwName].GlobalIPv6Address = iface.AddressIPv6().IP.String()
		networkSettings.Networks[nwName].GlobalIPv6PrefixLen = onesv6
	}

	return nil
}

// buildJoinOptions builds endpoint Join options from a given network.
func buildJoinOptions(networkSettings *internalnetwork.Settings, n interface{ Name() string }) ([]libnetwork.EndpointOption, error) {
	var joinOptions []libnetwork.EndpointOption
	if epConfig, ok := networkSettings.Networks[n.Name()]; ok {
		for _, str := range epConfig.Links {
			name, alias, err := opts.ParseLink(str)
			if err != nil {
				return nil, err
			}
			joinOptions = append(joinOptions, libnetwork.CreateOptionAlias(name, alias))
		}
		for k, v := range epConfig.DriverOpts {
			joinOptions = append(joinOptions, libnetwork.EndpointOptionGeneric(options.Generic{k: v}))
		}
	}

	return joinOptions, nil
}
