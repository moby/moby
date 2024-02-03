// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.19

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/containerd/log"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/multierror"
	"github.com/docker/docker/internal/sliceutil"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/go-connections/nat"
)

func ipAddresses(ips []net.IP) []string {
	var addrs []string
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}
	return addrs
}

func (daemon *Daemon) buildSandboxOptions(cfg *config.Config, container *container.Container) ([]libnetwork.SandboxOption, error) {
	var sboxOptions []libnetwork.SandboxOption
	sboxOptions = append(sboxOptions, libnetwork.OptionHostname(container.Config.Hostname), libnetwork.OptionDomainname(container.Config.Domainname))

	if container.HostConfig.NetworkMode.IsHost() {
		sboxOptions = append(sboxOptions, libnetwork.OptionUseDefaultSandbox())
	} else {
		// OptionUseExternalKey is mandatory for userns support.
		// But optional for non-userns support
		sboxOptions = append(sboxOptions, libnetwork.OptionUseExternalKey())
	}

	if err := setupPathsAndSandboxOptions(container, cfg, &sboxOptions); err != nil {
		return nil, err
	}

	if len(container.HostConfig.DNS) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNS(container.HostConfig.DNS))
	} else if len(cfg.DNS) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNS(ipAddresses(cfg.DNS)))
	}
	if len(container.HostConfig.DNSSearch) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSSearch(container.HostConfig.DNSSearch))
	} else if len(cfg.DNSSearch) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSSearch(cfg.DNSSearch))
	}
	if len(container.HostConfig.DNSOptions) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSOptions(container.HostConfig.DNSOptions))
	} else if len(cfg.DNSOptions) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSOptions(cfg.DNSOptions))
	}

	for _, extraHost := range container.HostConfig.ExtraHosts {
		// allow IPv6 addresses in extra hosts; only split on first ":"
		if _, err := opts.ValidateExtraHost(extraHost); err != nil {
			return nil, err
		}
		host, ip, _ := strings.Cut(extraHost, ":")
		// If the IP Address is a string called "host-gateway", replace this
		// value with the IP address stored in the daemon level HostGatewayIP
		// config variable
		if ip == opts.HostGatewayName {
			gateway := cfg.HostGatewayIP.String()
			if gateway == "" {
				return nil, fmt.Errorf("unable to derive the IP value for host-gateway")
			}
			ip = gateway
		}
		sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(host, ip))
	}

	bindings := make(nat.PortMap)
	if container.HostConfig.PortBindings != nil {
		for p, b := range container.HostConfig.PortBindings {
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
	ports := make([]nat.Port, 0, len(container.Config.ExposedPorts))
	for p := range container.Config.ExposedPorts {
		ports = append(ports, p)
	}
	nat.SortPortMap(ports, bindings)

	var (
		publishedPorts []types.PortBinding
		exposedPorts   []types.TransportPort
	)
	for _, port := range ports {
		portProto := types.ParseProtocol(port.Proto())
		portNum := uint16(port.Int())
		exposedPorts = append(exposedPorts, types.TransportPort{
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
				return nil, fmt.Errorf("Error parsing HostPort value(%s):%v", binding.HostPort, err)
			}
			publishedPorts = append(publishedPorts, types.PortBinding{
				Proto:       portProto,
				Port:        portNum,
				HostIP:      net.ParseIP(binding.HostIP),
				HostPort:    uint16(portStart),
				HostPortEnd: uint16(portEnd),
			})
		}

		if container.HostConfig.PublishAllPorts && len(bindings[port]) == 0 {
			publishedPorts = append(publishedPorts, types.PortBinding{
				Proto: portProto,
				Port:  portNum,
			})
		}
	}

	sboxOptions = append(sboxOptions, libnetwork.OptionPortMapping(publishedPorts), libnetwork.OptionExposedPorts(exposedPorts))

	// Legacy Link feature is supported only for the default bridge network.
	// return if this call to build join options is not for default bridge network
	// Legacy Link is only supported by docker run --link
	defaultNetName := runconfig.DefaultDaemonNetworkMode().NetworkName()
	bridgeSettings, ok := container.NetworkSettings.Networks[defaultNetName]
	if !ok || bridgeSettings.EndpointSettings == nil || bridgeSettings.EndpointID == "" {
		return sboxOptions, nil
	}

	var (
		childEndpoints []string
		cEndpointID    string
	)
	for linkAlias, child := range daemon.children(container) {
		if !isLinkable(child) {
			return nil, fmt.Errorf("Cannot link to %s, as it does not belong to the default network", child.Name)
		}
		_, alias := path.Split(linkAlias)
		// allow access to the linked container via the alias, real name, and container hostname
		aliasList := alias + " " + child.Config.Hostname
		// only add the name if alias isn't equal to the name
		if alias != child.Name[1:] {
			aliasList = aliasList + " " + child.Name[1:]
		}
		defaultNW := child.NetworkSettings.Networks[defaultNetName]
		if defaultNW.IPAddress != "" {
			sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(aliasList, defaultNW.IPAddress))
		}
		if defaultNW.GlobalIPv6Address != "" {
			sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(aliasList, defaultNW.GlobalIPv6Address))
		}
		cEndpointID = defaultNW.EndpointID
		if cEndpointID != "" {
			childEndpoints = append(childEndpoints, cEndpointID)
		}
	}

	var parentEndpoints []string
	for alias, parent := range daemon.parents(container) {
		if cfg.DisableBridge || !container.HostConfig.NetworkMode.IsPrivate() {
			continue
		}

		_, alias = path.Split(alias)
		log.G(context.TODO()).Debugf("Update /etc/hosts of %s for alias %s with ip %s", parent.ID, alias, bridgeSettings.IPAddress)
		sboxOptions = append(sboxOptions, libnetwork.OptionParentUpdate(parent.ID, alias, bridgeSettings.IPAddress))
		if cEndpointID != "" {
			parentEndpoints = append(parentEndpoints, cEndpointID)
		}
	}

	sboxOptions = append(sboxOptions, libnetwork.OptionGeneric(options.Generic{
		netlabel.GenericData: options.Generic{
			"ParentEndpoints": parentEndpoints,
			"ChildEndpoints":  childEndpoints,
		},
	}))
	return sboxOptions, nil
}

func (daemon *Daemon) updateNetworkSettings(container *container.Container, n *libnetwork.Network, endpointConfig *networktypes.EndpointSettings) error {
	if container.NetworkSettings == nil {
		container.NetworkSettings = &network.Settings{}
	}
	if container.NetworkSettings.Networks == nil {
		container.NetworkSettings.Networks = make(map[string]*network.EndpointSettings)
	}

	if !container.HostConfig.NetworkMode.IsHost() && containertypes.NetworkMode(n.Type()).IsHost() {
		return runconfig.ErrConflictHostNetwork
	}

	for s, v := range container.NetworkSettings.Networks {
		sn, err := daemon.FindNetwork(getNetworkID(s, v.EndpointSettings))
		if err != nil {
			continue
		}

		if sn.Name() == n.Name() {
			// If the network scope is swarm, then this
			// is an attachable network, which may not
			// be locally available previously.
			// So always update.
			if n.Scope() == scope.Swarm {
				continue
			}
			// Avoid duplicate config
			return nil
		}
		if !containertypes.NetworkMode(sn.Type()).IsPrivate() ||
			!containertypes.NetworkMode(n.Type()).IsPrivate() {
			return runconfig.ErrConflictSharedNetwork
		}
		if containertypes.NetworkMode(sn.Name()).IsNone() ||
			containertypes.NetworkMode(n.Name()).IsNone() {
			return runconfig.ErrConflictNoNetwork
		}
	}

	container.NetworkSettings.Networks[n.Name()] = &network.EndpointSettings{
		EndpointSettings: endpointConfig,
	}

	return nil
}

func (daemon *Daemon) updateEndpointNetworkSettings(cfg *config.Config, container *container.Container, n *libnetwork.Network, ep *libnetwork.Endpoint) error {
	if err := buildEndpointInfo(container.NetworkSettings, n, ep); err != nil {
		return err
	}

	if container.HostConfig.NetworkMode == runconfig.DefaultDaemonNetworkMode() {
		container.NetworkSettings.Bridge = cfg.BridgeConfig.Iface
	}

	return nil
}

// UpdateNetwork is used to update the container's network (e.g. when linked containers
// get removed/unlinked).
func (daemon *Daemon) updateNetwork(cfg *config.Config, container *container.Container) error {
	var (
		start = time.Now()
		ctrl  = daemon.netController
		sid   = container.NetworkSettings.SandboxID
	)

	sb, err := ctrl.SandboxByID(sid)
	if err != nil {
		return fmt.Errorf("error locating sandbox id %s: %v", sid, err)
	}

	// Find if container is connected to the default bridge network
	var n *libnetwork.Network
	for name, v := range container.NetworkSettings.Networks {
		sn, err := daemon.FindNetwork(getNetworkID(name, v.EndpointSettings))
		if err != nil {
			continue
		}
		if sn.Name() == runconfig.DefaultDaemonNetworkMode().NetworkName() {
			n = sn
			break
		}
	}

	if n == nil {
		// Not connected to the default bridge network; Nothing to do
		return nil
	}

	sbOptions, err := daemon.buildSandboxOptions(cfg, container)
	if err != nil {
		return fmt.Errorf("Update network failed: %v", err)
	}

	if err := sb.Refresh(sbOptions...); err != nil {
		return fmt.Errorf("Update network failed: Failure in refresh sandbox %s: %v", sid, err)
	}

	networkActions.WithValues("update").UpdateSince(start)

	return nil
}

func (daemon *Daemon) findAndAttachNetwork(container *container.Container, idOrName string, epConfig *networktypes.EndpointSettings) (*libnetwork.Network, *networktypes.NetworkingConfig, error) {
	id := getNetworkID(idOrName, epConfig)

	n, err := daemon.FindNetwork(id)
	if err != nil {
		// We should always be able to find the network for a managed container.
		if container.Managed {
			return nil, nil, err
		}
	}

	// If we found a network and if it is not dynamically created
	// we should never attempt to attach to that network here.
	if n != nil {
		if container.Managed || !n.Dynamic() {
			return n, nil, nil
		}
		// Throw an error if the container is already attached to the network
		if container.NetworkSettings.Networks != nil {
			networkName := n.Name()
			containerName := strings.TrimPrefix(container.Name, "/")
			if nw, ok := container.NetworkSettings.Networks[networkName]; ok && nw.EndpointID != "" {
				err := fmt.Errorf("%s is already attached to network %s", containerName, networkName)
				return n, nil, errdefs.Conflict(err)
			}
		}
	}

	var addresses []string
	if epConfig != nil && epConfig.IPAMConfig != nil {
		if epConfig.IPAMConfig.IPv4Address != "" {
			addresses = append(addresses, epConfig.IPAMConfig.IPv4Address)
		}
		if epConfig.IPAMConfig.IPv6Address != "" {
			addresses = append(addresses, epConfig.IPAMConfig.IPv6Address)
		}
	}

	if n == nil && daemon.attachableNetworkLock != nil {
		daemon.attachableNetworkLock.Lock(id)
		defer daemon.attachableNetworkLock.Unlock(id)
	}

	retryCount := 0
	var nwCfg *networktypes.NetworkingConfig
	for {
		// In all other cases, attempt to attach to the network to
		// trigger attachment in the swarm cluster manager.
		if daemon.clusterProvider != nil {
			var err error
			nwCfg, err = daemon.clusterProvider.AttachNetwork(id, container.ID, addresses)
			if err != nil {
				return nil, nil, err
			}
		}

		n, err = daemon.FindNetwork(id)
		if err != nil {
			if daemon.clusterProvider != nil {
				if err := daemon.clusterProvider.DetachNetwork(id, container.ID); err != nil {
					log.G(context.TODO()).Warnf("Could not rollback attachment for container %s to network %s: %v", container.ID, idOrName, err)
				}
			}

			// Retry network attach again if we failed to
			// find the network after successful
			// attachment because the only reason that
			// would happen is if some other container
			// attached to the swarm scope network went down
			// and removed the network while we were in
			// the process of attaching.
			if nwCfg != nil {
				if _, ok := err.(libnetwork.ErrNoSuchNetwork); ok {
					if retryCount >= 5 {
						return nil, nil, fmt.Errorf("could not find network %s after successful attachment", idOrName)
					}
					retryCount++
					continue
				}
			}

			return nil, nil, err
		}

		break
	}

	// This container has attachment to a swarm scope
	// network. Update the container network settings accordingly.
	container.NetworkSettings.HasSwarmEndpoint = true
	return n, nwCfg, nil
}

// updateContainerNetworkSettings updates the network settings
func (daemon *Daemon) updateContainerNetworkSettings(container *container.Container, endpointsConfig map[string]*networktypes.EndpointSettings) {
	var n *libnetwork.Network

	mode := container.HostConfig.NetworkMode
	if container.Config.NetworkDisabled || mode.IsContainer() {
		return
	}

	networkName := mode.NetworkName()
	if mode.IsDefault() {
		networkName = daemon.netController.Config().DefaultNetwork
	}

	if mode.IsUserDefined() {
		var err error

		n, err = daemon.FindNetwork(networkName)
		if err == nil {
			networkName = n.Name()
		}
	}

	if container.NetworkSettings == nil {
		container.NetworkSettings = &network.Settings{}
	}

	if len(endpointsConfig) > 0 {
		if container.NetworkSettings.Networks == nil {
			container.NetworkSettings.Networks = make(map[string]*network.EndpointSettings)
		}

		for name, epConfig := range endpointsConfig {
			container.NetworkSettings.Networks[name] = &network.EndpointSettings{
				EndpointSettings: epConfig,
				// At this point, during container creation, epConfig.MacAddress is the
				// configured value from the API. If there is no configured value, the
				// same field will later be used to store a generated MAC address. So,
				// remember the requested address now.
				DesiredMacAddress: epConfig.MacAddress,
			}
		}
	}

	if container.NetworkSettings.Networks == nil {
		container.NetworkSettings.Networks = make(map[string]*network.EndpointSettings)
		container.NetworkSettings.Networks[networkName] = &network.EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{},
		}
	}

	// Convert any settings added by client in default name to
	// engine's default network name key
	if mode.IsDefault() {
		if nConf, ok := container.NetworkSettings.Networks[mode.NetworkName()]; ok {
			container.NetworkSettings.Networks[networkName] = nConf
			delete(container.NetworkSettings.Networks, mode.NetworkName())
		}
	}

	if !mode.IsUserDefined() {
		return
	}
	// Make sure to internally store the per network endpoint config by network name
	if _, ok := container.NetworkSettings.Networks[networkName]; ok {
		return
	}

	if n != nil {
		if nwConfig, ok := container.NetworkSettings.Networks[n.ID()]; ok {
			container.NetworkSettings.Networks[networkName] = nwConfig
			delete(container.NetworkSettings.Networks, n.ID())
			return
		}
	}
}

func (daemon *Daemon) allocateNetwork(cfg *config.Config, container *container.Container) (retErr error) {
	if daemon.netController == nil {
		return nil
	}

	start := time.Now()

	// Cleanup any stale sandbox left over due to ungraceful daemon shutdown
	if err := daemon.netController.SandboxDestroy(container.ID); err != nil {
		log.G(context.TODO()).WithError(err).Errorf("failed to cleanup up stale network sandbox for container %s", container.ID)
	}

	if container.Config.NetworkDisabled || container.HostConfig.NetworkMode.IsContainer() {
		return nil
	}

	updateSettings := false
	if len(container.NetworkSettings.Networks) == 0 {
		daemon.updateContainerNetworkSettings(container, nil)
		updateSettings = true
	}

	// always connect default network first since only default
	// network mode support link and we need do some setting
	// on sandbox initialize for link, but the sandbox only be initialized
	// on first network connecting.
	defaultNetName := runconfig.DefaultDaemonNetworkMode().NetworkName()
	if nConf, ok := container.NetworkSettings.Networks[defaultNetName]; ok {
		cleanOperationalData(nConf)
		if err := daemon.connectToNetwork(cfg, container, defaultNetName, nConf, updateSettings); err != nil {
			return err
		}
	}

	// the intermediate map is necessary because "connectToNetwork" modifies "container.NetworkSettings.Networks"
	networks := make(map[string]*network.EndpointSettings)
	for n, epConf := range container.NetworkSettings.Networks {
		if n == defaultNetName {
			continue
		}

		networks[n] = epConf
	}

	for netName, epConf := range networks {
		cleanOperationalData(epConf)
		if err := daemon.connectToNetwork(cfg, container, netName, epConf, updateSettings); err != nil {
			return err
		}
	}

	// If the container is not to be connected to any network,
	// create its network sandbox now if not present
	if len(networks) == 0 {
		if _, err := daemon.netController.GetSandbox(container.ID); err != nil {
			if !errdefs.IsNotFound(err) {
				return err
			}

			sbOptions, err := daemon.buildSandboxOptions(cfg, container)
			if err != nil {
				return err
			}
			sb, err := daemon.netController.NewSandbox(container.ID, sbOptions...)
			if err != nil {
				return err
			}
			setNetworkSandbox(container, sb)
			defer func() {
				if retErr != nil {
					sb.Delete()
				}
			}()
		}
	}

	if _, err := container.WriteHostConfig(); err != nil {
		return err
	}
	networkActions.WithValues("allocate").UpdateSince(start)
	return nil
}

// validateEndpointSettings checks whether the given epConfig is valid. The nw parameter can be nil, in which case it
// won't try to check if the endpoint IP addresses are within network's subnets.
func validateEndpointSettings(nw *libnetwork.Network, nwName string, epConfig *networktypes.EndpointSettings) error {
	if epConfig == nil {
		return nil
	}

	ipamConfig := &networktypes.EndpointIPAMConfig{}
	if epConfig.IPAMConfig != nil {
		ipamConfig = epConfig.IPAMConfig
	}

	var errs []error

	// TODO(aker): move this into api/types/network/endpoint.go once enableIPOnPredefinedNetwork and
	//  serviceDiscoveryOnDefaultNetwork are removed.
	if !containertypes.NetworkMode(nwName).IsUserDefined() {
		hasStaticAddresses := ipamConfig.IPv4Address != "" || ipamConfig.IPv6Address != ""
		// On Linux, user specified IP address is accepted only by networks with user specified subnets.
		if hasStaticAddresses && !enableIPOnPredefinedNetwork() {
			errs = append(errs, runconfig.ErrUnsupportedNetworkAndIP)
		}
		if len(epConfig.Aliases) > 0 && !serviceDiscoveryOnDefaultNetwork() {
			errs = append(errs, runconfig.ErrUnsupportedNetworkAndAlias)
		}
	}

	// TODO(aker): add a proper multierror.Append
	if err := ipamConfig.Validate(); err != nil {
		errs = append(errs, err.(interface{ Unwrap() []error }).Unwrap()...)
	}

	if nw != nil {
		_, _, v4Configs, v6Configs := nw.IpamConfig()

		var nwIPv4Subnets, nwIPv6Subnets []networktypes.NetworkSubnet
		for _, nwIPAMConfig := range v4Configs {
			nwIPv4Subnets = append(nwIPv4Subnets, nwIPAMConfig)
		}
		for _, nwIPAMConfig := range v6Configs {
			nwIPv6Subnets = append(nwIPv6Subnets, nwIPAMConfig)
		}

		// TODO(aker): add a proper multierror.Append
		if err := ipamConfig.IsInRange(nwIPv4Subnets, nwIPv6Subnets); err != nil {
			errs = append(errs, err.(interface{ Unwrap() []error }).Unwrap()...)
		}
	}

	if epConfig.MacAddress != "" {
		_, err := net.ParseMAC(epConfig.MacAddress)
		if err != nil {
			return fmt.Errorf("invalid MAC address %s", epConfig.MacAddress)
		}
	}

	if err := multierror.Join(errs...); err != nil {
		return fmt.Errorf("invalid endpoint settings:\n%w", err)
	}

	return nil
}

// cleanOperationalData resets the operational data from the passed endpoint settings
func cleanOperationalData(es *network.EndpointSettings) {
	es.EndpointID = ""
	es.Gateway = ""
	es.IPAddress = ""
	es.IPPrefixLen = 0
	es.IPv6Gateway = ""
	es.GlobalIPv6Address = ""
	es.GlobalIPv6PrefixLen = 0
	es.MacAddress = ""
	if es.IPAMOperational {
		es.IPAMConfig = nil
	}
}

func (daemon *Daemon) updateNetworkConfig(container *container.Container, n *libnetwork.Network, endpointConfig *networktypes.EndpointSettings, updateSettings bool) error {
	if containertypes.NetworkMode(n.Name()).IsUserDefined() {
		endpointConfig.DNSNames = buildEndpointDNSNames(container, endpointConfig.Aliases)
	}

	if err := validateEndpointSettings(n, n.Name(), endpointConfig); err != nil {
		return errdefs.InvalidParameter(err)
	}

	if updateSettings {
		if err := daemon.updateNetworkSettings(container, n, endpointConfig); err != nil {
			return err
		}
	}
	return nil
}

// buildEndpointDNSNames constructs the list of DNSNames that should be assigned to a given endpoint. The order within
// the returned slice is important as the first entry will be used to generate the PTR records (for IPv4 and v6)
// associated to this endpoint.
func buildEndpointDNSNames(ctr *container.Container, aliases []string) []string {
	var dnsNames []string

	if ctr.Name != "" {
		dnsNames = append(dnsNames, strings.TrimPrefix(ctr.Name, "/"))
	}

	dnsNames = append(dnsNames, aliases...)

	if ctr.ID != "" {
		dnsNames = append(dnsNames, stringid.TruncateID(ctr.ID))
	}

	if ctr.Config.Hostname != "" {
		dnsNames = append(dnsNames, ctr.Config.Hostname)
	}

	return sliceutil.Dedup(dnsNames)
}

func (daemon *Daemon) connectToNetwork(cfg *config.Config, container *container.Container, idOrName string, endpointConfig *network.EndpointSettings, updateSettings bool) (retErr error) {
	start := time.Now()
	if container.HostConfig.NetworkMode.IsContainer() {
		return runconfig.ErrConflictSharedNetwork
	}
	if cfg.DisableBridge && containertypes.NetworkMode(idOrName).IsBridge() {
		container.Config.NetworkDisabled = true
		return nil
	}
	if endpointConfig == nil {
		endpointConfig = &network.EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{},
		}
	}

	n, nwCfg, err := daemon.findAndAttachNetwork(container, idOrName, endpointConfig.EndpointSettings)
	if err != nil {
		return err
	}
	if n == nil {
		return nil
	}
	nwName := n.Name()

	if idOrName != container.HostConfig.NetworkMode.NetworkName() {
		if err := daemon.normalizeNetMode(container); err != nil {
			return err
		}
	}

	endpointConfig.IPAMOperational = false
	if nwCfg != nil {
		if epConfig, ok := nwCfg.EndpointsConfig[nwName]; ok {
			if endpointConfig.IPAMConfig == nil || (endpointConfig.IPAMConfig.IPv4Address == "" && endpointConfig.IPAMConfig.IPv6Address == "" && len(endpointConfig.IPAMConfig.LinkLocalIPs) == 0) {
				endpointConfig.IPAMOperational = true
			}

			// copy IPAMConfig and NetworkID from epConfig via AttachNetwork
			endpointConfig.IPAMConfig = epConfig.IPAMConfig
			endpointConfig.NetworkID = epConfig.NetworkID
		}
	}

	if err := daemon.updateNetworkConfig(container, n, endpointConfig.EndpointSettings, updateSettings); err != nil {
		return err
	}

	// TODO(thaJeztah): should this fail early if no sandbox was found?
	sb, _ := daemon.netController.GetSandbox(container.ID)
	createOptions, err := buildCreateEndpointOptions(container, n, endpointConfig, sb, ipAddresses(cfg.DNS))
	if err != nil {
		return err
	}

	endpointName := strings.TrimPrefix(container.Name, "/")
	ep, err := n.CreateEndpoint(endpointName, createOptions...)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			if err := ep.Delete(false); err != nil {
				log.G(context.TODO()).Warnf("Could not rollback container connection to network %s", idOrName)
			}
		}
	}()
	container.NetworkSettings.Networks[nwName] = endpointConfig

	delete(container.NetworkSettings.Networks, n.ID())

	if err := daemon.updateEndpointNetworkSettings(cfg, container, n, ep); err != nil {
		return err
	}

	if sb == nil {
		sbOptions, err := daemon.buildSandboxOptions(cfg, container)
		if err != nil {
			return err
		}
		sb, err = daemon.netController.NewSandbox(container.ID, sbOptions...)
		if err != nil {
			return err
		}

		setNetworkSandbox(container, sb)
	}

	joinOptions, err := buildJoinOptions(container.NetworkSettings, n)
	if err != nil {
		return err
	}

	if err := ep.Join(sb, joinOptions...); err != nil {
		return err
	}

	if !container.Managed {
		// add container name/alias to DNS
		if err := daemon.ActivateContainerServiceBinding(container.Name); err != nil {
			return fmt.Errorf("Activate container service binding for %s failed: %v", container.Name, err)
		}
	}

	if err := updateJoinInfo(container.NetworkSettings, n, ep); err != nil {
		return fmt.Errorf("Updating join info failed: %v", err)
	}

	container.NetworkSettings.Ports = getPortMapInfo(sb)

	daemon.LogNetworkEventWithAttributes(n, events.ActionConnect, map[string]string{"container": container.ID})
	networkActions.WithValues("connect").UpdateSince(start)
	return nil
}

func updateJoinInfo(networkSettings *network.Settings, n *libnetwork.Network, ep *libnetwork.Endpoint) error {
	if ep == nil {
		return errors.New("invalid enppoint whhile building portmap info")
	}

	if networkSettings == nil {
		return errors.New("invalid network settings while building port map info")
	}

	if len(networkSettings.Ports) == 0 {
		pm, err := getEndpointPortMapInfo(ep)
		if err != nil {
			return err
		}
		networkSettings.Ports = pm
	}

	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return nil
	}
	if epInfo.Gateway() != nil {
		networkSettings.Networks[n.Name()].Gateway = epInfo.Gateway().String()
	}
	if epInfo.GatewayIPv6().To16() != nil {
		networkSettings.Networks[n.Name()].IPv6Gateway = epInfo.GatewayIPv6().String()
	}
	return nil
}

// ForceEndpointDelete deletes an endpoint from a network forcefully
func (daemon *Daemon) ForceEndpointDelete(name string, networkName string) error {
	n, err := daemon.FindNetwork(networkName)
	if err != nil {
		return err
	}

	ep, err := n.EndpointByName(name)
	if err != nil {
		return err
	}
	return ep.Delete(true)
}

func (daemon *Daemon) disconnectFromNetwork(container *container.Container, n *libnetwork.Network, force bool) error {
	var (
		ep   *libnetwork.Endpoint
		sbox *libnetwork.Sandbox
	)
	n.WalkEndpoints(func(current *libnetwork.Endpoint) bool {
		epInfo := current.Info()
		if epInfo == nil {
			return false
		}
		if sb := epInfo.Sandbox(); sb != nil {
			if sb.ContainerID() == container.ID {
				ep = current
				sbox = sb
				return true
			}
		}
		return false
	})

	if ep == nil {
		if force {
			var err error
			ep, err = n.EndpointByName(strings.TrimPrefix(container.Name, "/"))
			if err != nil {
				return err
			}
			return ep.Delete(force)
		}
		return fmt.Errorf("container %s is not connected to network %s", container.ID, n.Name())
	}

	if err := ep.Leave(sbox); err != nil {
		return fmt.Errorf("container %s failed to leave network %s: %v", container.ID, n.Name(), err)
	}

	container.NetworkSettings.Ports = getPortMapInfo(sbox)

	if err := ep.Delete(false); err != nil {
		return fmt.Errorf("endpoint delete failed for container %s on network %s: %v", container.ID, n.Name(), err)
	}

	delete(container.NetworkSettings.Networks, n.Name())

	daemon.tryDetachContainerFromClusterNetwork(n, container)

	return nil
}

func (daemon *Daemon) tryDetachContainerFromClusterNetwork(network *libnetwork.Network, container *container.Container) {
	if !container.Managed && daemon.clusterProvider != nil && network.Dynamic() {
		if err := daemon.clusterProvider.DetachNetwork(network.Name(), container.ID); err != nil {
			log.G(context.TODO()).WithError(err).Warn("error detaching from network")
			if err := daemon.clusterProvider.DetachNetwork(network.ID(), container.ID); err != nil {
				log.G(context.TODO()).WithError(err).Warn("error detaching from network")
			}
		}
	}
	daemon.LogNetworkEventWithAttributes(network, events.ActionDisconnect, map[string]string{
		"container": container.ID,
	})
}

// normalizeNetMode checks whether the network mode references a network by a partial ID. In that case, it replaces the
// partial ID with the full network ID.
// TODO(aker): transform ID into name when the referenced network is one of the predefined.
func (daemon *Daemon) normalizeNetMode(container *container.Container) error {
	if container.HostConfig.NetworkMode.IsUserDefined() {
		netMode := container.HostConfig.NetworkMode.NetworkName()
		nw, err := daemon.FindNetwork(netMode)
		if err != nil {
			return fmt.Errorf("could not find a network matching network mode %s: %w", netMode, err)
		}

		if netMode != nw.ID() && netMode != nw.Name() {
			container.HostConfig.NetworkMode = containertypes.NetworkMode(nw.ID())
		}
	}

	return nil
}

func (daemon *Daemon) initializeNetworking(cfg *config.Config, container *container.Container) error {
	if container.HostConfig.NetworkMode.IsContainer() {
		// we need to get the hosts files from the container to join
		nc, err := daemon.getNetworkedContainer(container.ID, container.HostConfig.NetworkMode.ConnectedContainer())
		if err != nil {
			return err
		}

		err = daemon.initializeNetworkingPaths(container, nc)
		if err != nil {
			return err
		}

		container.Config.Hostname = nc.Config.Hostname
		container.Config.Domainname = nc.Config.Domainname
		return nil
	}

	if container.HostConfig.NetworkMode.IsHost() && container.Config.Hostname == "" {
		hn, err := os.Hostname()
		if err != nil {
			return err
		}
		container.Config.Hostname = hn
	}

	if err := daemon.allocateNetwork(cfg, container); err != nil {
		return err
	}

	return container.BuildHostnameFile()
}

func (daemon *Daemon) getNetworkedContainer(containerID, connectedContainerID string) (*container.Container, error) {
	nc, err := daemon.GetContainer(connectedContainerID)
	if err != nil {
		return nil, err
	}
	if containerID == nc.ID {
		return nil, fmt.Errorf("cannot join own network")
	}
	if !nc.IsRunning() {
		return nil, errdefs.Conflict(fmt.Errorf("cannot join network of a non running container: %s", connectedContainerID))
	}
	if nc.IsRestarting() {
		return nil, errContainerIsRestarting(connectedContainerID)
	}
	return nc, nil
}

func (daemon *Daemon) releaseNetwork(container *container.Container) {
	start := time.Now()
	// If live-restore is enabled, the daemon cleans up dead containers when it starts up. In that case, the
	// netController hasn't been initialized yet and so we can't proceed.
	// TODO(aker): If we hit this case, the endpoint state won't be cleaned up (ie. no call to cleanOperationalData).
	if daemon.netController == nil {
		return
	}
	// If the container uses the network namespace of another container, it doesn't own it -- nothing to do here.
	if container.HostConfig.NetworkMode.IsContainer() {
		return
	}
	if container.NetworkSettings == nil {
		return
	}

	container.NetworkSettings.Ports = nil
	sid := container.NetworkSettings.SandboxID
	if sid == "" {
		return
	}

	var networks []*libnetwork.Network
	for n, epSettings := range container.NetworkSettings.Networks {
		if nw, err := daemon.FindNetwork(getNetworkID(n, epSettings.EndpointSettings)); err == nil {
			networks = append(networks, nw)
		}

		if epSettings.EndpointSettings == nil {
			continue
		}

		cleanOperationalData(epSettings)
	}

	sb, err := daemon.netController.SandboxByID(sid)
	if err != nil {
		log.G(context.TODO()).Warnf("error locating sandbox id %s: %v", sid, err)
		return
	}

	if err := sb.Delete(); err != nil {
		log.G(context.TODO()).Errorf("Error deleting sandbox id %s for container %s: %v", sid, container.ID, err)
	}

	for _, nw := range networks {
		daemon.tryDetachContainerFromClusterNetwork(nw, container)
	}
	networkActions.WithValues("release").UpdateSince(start)
}

func errRemovalContainer(containerID string) error {
	return fmt.Errorf("Container %s is marked for removal and cannot be connected or disconnected to the network", containerID)
}

// ConnectToNetwork connects a container to a network
func (daemon *Daemon) ConnectToNetwork(container *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings) error {
	if endpointConfig == nil {
		endpointConfig = &networktypes.EndpointSettings{}
	}
	container.Lock()
	defer container.Unlock()

	if !container.Running {
		if container.RemovalInProgress || container.Dead {
			return errRemovalContainer(container.ID)
		}

		n, err := daemon.FindNetwork(idOrName)
		if err == nil && n != nil {
			if err := daemon.updateNetworkConfig(container, n, endpointConfig, true); err != nil {
				return err
			}
		} else {
			container.NetworkSettings.Networks[idOrName] = &network.EndpointSettings{
				EndpointSettings: endpointConfig,
			}
		}
	} else {
		epc := &network.EndpointSettings{
			EndpointSettings: endpointConfig,
		}
		if err := daemon.connectToNetwork(&daemon.config().Config, container, idOrName, epc, true); err != nil {
			return err
		}
	}

	return container.CheckpointTo(daemon.containersReplica)
}

// DisconnectFromNetwork disconnects container from network n.
func (daemon *Daemon) DisconnectFromNetwork(container *container.Container, networkName string, force bool) error {
	n, err := daemon.FindNetwork(networkName)
	container.Lock()
	defer container.Unlock()

	if !container.Running || (err != nil && force) {
		if container.RemovalInProgress || container.Dead {
			return errRemovalContainer(container.ID)
		}
		// In case networkName is resolved we will use n.Name()
		// this will cover the case where network id is passed.
		if n != nil {
			networkName = n.Name()
		}
		if _, ok := container.NetworkSettings.Networks[networkName]; !ok {
			return fmt.Errorf("container %s is not connected to the network %s", container.ID, networkName)
		}
		delete(container.NetworkSettings.Networks, networkName)
	} else if err == nil {
		if container.HostConfig.NetworkMode.IsHost() && containertypes.NetworkMode(n.Type()).IsHost() {
			return runconfig.ErrConflictHostNetwork
		}

		if err := daemon.disconnectFromNetwork(container, n, false); err != nil {
			return err
		}
	} else {
		return err
	}

	if err := container.CheckpointTo(daemon.containersReplica); err != nil {
		return err
	}

	if n != nil {
		daemon.LogNetworkEventWithAttributes(n, events.ActionDisconnect, map[string]string{
			"container": container.ID,
		})
	}

	return nil
}

// ActivateContainerServiceBinding puts this container into load balancer active rotation and DNS response
func (daemon *Daemon) ActivateContainerServiceBinding(containerName string) error {
	ctr, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}
	sb, err := daemon.netController.GetSandbox(ctr.ID)
	if err != nil {
		return fmt.Errorf("failed to activate service binding for container %s: %w", containerName, err)
	}
	return sb.EnableService()
}

// DeactivateContainerServiceBinding removes this container from load balancer active rotation, and DNS response
func (daemon *Daemon) DeactivateContainerServiceBinding(containerName string) error {
	ctr, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}
	sb, err := daemon.netController.GetSandbox(ctr.ID)
	if err != nil {
		// If the network sandbox is not found, then there is nothing to deactivate
		log.G(context.TODO()).WithError(err).Debugf("Could not find network sandbox for container %s on service binding deactivation request", containerName)
		return nil
	}
	return sb.DisableService()
}

func getNetworkID(name string, endpointSettings *networktypes.EndpointSettings) string {
	// We only want to prefer NetworkID for user defined networks.
	// For systems like bridge, none, etc. the name is preferred (otherwise restart may cause issues)
	if containertypes.NetworkMode(name).IsUserDefined() && endpointSettings != nil && endpointSettings.NetworkID != "" {
		return endpointSettings.NetworkID
	}
	return name
}

// setNetworkSandbox updates the sandbox ID and Key.
func setNetworkSandbox(c *container.Container, sb *libnetwork.Sandbox) {
	c.NetworkSettings.SandboxID = sb.ID()
	c.NetworkSettings.SandboxKey = sb.Key()
}
