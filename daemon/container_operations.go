// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
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
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/go-connections/nat"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func ipAddresses(ips []net.IP) []string {
	var addrs []string
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}
	return addrs
}

func buildSandboxOptions(cfg *config.Config, ctr *container.Container) ([]libnetwork.SandboxOption, error) {
	var sboxOptions []libnetwork.SandboxOption
	sboxOptions = append(sboxOptions, libnetwork.OptionHostname(ctr.Config.Hostname), libnetwork.OptionDomainname(ctr.Config.Domainname))

	if ctr.HostConfig.NetworkMode.IsHost() {
		sboxOptions = append(sboxOptions, libnetwork.OptionUseDefaultSandbox())
	} else {
		// OptionUseExternalKey is mandatory for userns support.
		// But optional for non-userns support
		sboxOptions = append(sboxOptions, libnetwork.OptionUseExternalKey())
	}

	// Add platform-specific Sandbox options.
	if err := buildSandboxPlatformOptions(ctr, cfg, &sboxOptions); err != nil {
		return nil, err
	}

	if len(ctr.HostConfig.DNS) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNS(ctr.HostConfig.DNS))
	} else if len(cfg.DNS) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNS(ipAddresses(cfg.DNS)))
	}
	if len(ctr.HostConfig.DNSSearch) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSSearch(ctr.HostConfig.DNSSearch))
	} else if len(cfg.DNSSearch) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSSearch(cfg.DNSSearch))
	}
	if len(ctr.HostConfig.DNSOptions) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSOptions(ctr.HostConfig.DNSOptions))
	} else if len(cfg.DNSOptions) > 0 {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSOptions(cfg.DNSOptions))
	}

	for _, extraHost := range ctr.HostConfig.ExtraHosts {
		// allow IPv6 addresses in extra hosts; only split on first ":"
		if _, err := opts.ValidateExtraHost(extraHost); err != nil {
			return nil, err
		}
		host, ip, _ := strings.Cut(extraHost, ":")
		// If the IP Address is the literal string "host-gateway", replace this
		// value with the IP address(es) stored in the daemon level HostGatewayIP
		// config variable
		if ip == opts.HostGatewayName {
			if len(cfg.HostGatewayIPs) == 0 {
				return nil, fmt.Errorf("unable to derive the IP value for host-gateway")
			}
			for _, gip := range cfg.HostGatewayIPs {
				sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(host, gip.String()))
			}
		} else {
			sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(host, ip))
		}
	}

	bindings := make(nat.PortMap)
	if ctr.HostConfig.PortBindings != nil {
		for p, b := range ctr.HostConfig.PortBindings {
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
	ports := make([]nat.Port, 0, len(ctr.Config.ExposedPorts))
	for p := range ctr.Config.ExposedPorts {
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

		if ctr.HostConfig.PublishAllPorts && len(bindings[port]) == 0 {
			publishedPorts = append(publishedPorts, types.PortBinding{
				Proto: portProto,
				Port:  portNum,
			})
		}
	}

	sboxOptions = append(sboxOptions, libnetwork.OptionPortMapping(publishedPorts), libnetwork.OptionExposedPorts(exposedPorts))

	return sboxOptions, nil
}

func (daemon *Daemon) updateNetworkSettings(ctr *container.Container, n *libnetwork.Network, endpointConfig *networktypes.EndpointSettings) error {
	if ctr.NetworkSettings == nil {
		ctr.NetworkSettings = &network.Settings{}
	}
	if ctr.NetworkSettings.Networks == nil {
		ctr.NetworkSettings.Networks = make(map[string]*network.EndpointSettings)
	}

	if !ctr.HostConfig.NetworkMode.IsHost() && containertypes.NetworkMode(n.Type()).IsHost() {
		return runconfig.ErrConflictHostNetwork
	}

	for s, v := range ctr.NetworkSettings.Networks {
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

	ctr.NetworkSettings.Networks[n.Name()] = &network.EndpointSettings{
		EndpointSettings: endpointConfig,
	}

	return nil
}

func (daemon *Daemon) updateEndpointNetworkSettings(cfg *config.Config, ctr *container.Container, n *libnetwork.Network, ep *libnetwork.Endpoint) error {
	if err := buildEndpointInfo(ctr.NetworkSettings, n, ep); err != nil {
		return err
	}

	if ctr.HostConfig.NetworkMode == network.DefaultNetwork {
		ctr.NetworkSettings.Bridge = cfg.BridgeConfig.Iface
	}

	return nil
}

// UpdateNetwork is used to update the container's network (e.g. when linked containers
// get removed/unlinked).
func (daemon *Daemon) updateNetwork(cfg *config.Config, ctr *container.Container) error {
	var (
		start = time.Now()
		ctrl  = daemon.netController
		sid   = ctr.NetworkSettings.SandboxID
	)

	sb, err := ctrl.SandboxByID(sid)
	if err != nil {
		return fmt.Errorf("error locating sandbox id %s: %v", sid, err)
	}

	// Find if container is connected to the default bridge network
	var n *libnetwork.Network
	for name, v := range ctr.NetworkSettings.Networks {
		sn, err := daemon.FindNetwork(getNetworkID(name, v.EndpointSettings))
		if err != nil {
			continue
		}
		if sn.Name() == network.DefaultNetwork {
			n = sn
			break
		}
	}

	if n == nil {
		// Not connected to the default bridge network; Nothing to do
		return nil
	}

	sbOptions, err := buildSandboxOptions(cfg, ctr)
	if err != nil {
		return fmt.Errorf("Update network failed: %v", err)
	}

	if err := sb.Refresh(context.TODO(), sbOptions...); err != nil {
		return fmt.Errorf("Update network failed: Failure in refresh sandbox %s: %v", sid, err)
	}

	networkActions.WithValues("update").UpdateSince(start)

	return nil
}

func (daemon *Daemon) findAndAttachNetwork(ctr *container.Container, idOrName string, epConfig *networktypes.EndpointSettings) (*libnetwork.Network, *networktypes.NetworkingConfig, error) {
	id := getNetworkID(idOrName, epConfig)

	n, err := daemon.FindNetwork(id)
	if err != nil {
		// We should always be able to find the network for a managed container.
		if ctr.Managed {
			return nil, nil, err
		}
	}

	// If we found a network and if it is not dynamically created
	// we should never attempt to attach to that network here.
	if n != nil {
		if ctr.Managed || !n.Dynamic() {
			return n, nil, nil
		}
		// Throw an error if the container is already attached to the network
		if ctr.NetworkSettings.Networks != nil {
			networkName := n.Name()
			containerName := strings.TrimPrefix(ctr.Name, "/")
			if nw, ok := ctr.NetworkSettings.Networks[networkName]; ok && nw.EndpointID != "" {
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
			nwCfg, err = daemon.clusterProvider.AttachNetwork(id, ctr.ID, addresses)
			if err != nil {
				return nil, nil, err
			}
		}

		n, err = daemon.FindNetwork(id)
		if err != nil {
			if daemon.clusterProvider != nil {
				if err := daemon.clusterProvider.DetachNetwork(id, ctr.ID); err != nil {
					log.G(context.TODO()).Warnf("Could not rollback attachment for container %s to network %s: %v", ctr.ID, idOrName, err)
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
	ctr.NetworkSettings.HasSwarmEndpoint = true
	return n, nwCfg, nil
}

// updateContainerNetworkSettings updates the network settings
func (daemon *Daemon) updateContainerNetworkSettings(ctr *container.Container, endpointsConfig map[string]*networktypes.EndpointSettings) {
	var n *libnetwork.Network

	mode := ctr.HostConfig.NetworkMode
	if ctr.Config.NetworkDisabled || mode.IsContainer() {
		return
	}

	networkName := mode.NetworkName()

	if mode.IsUserDefined() {
		var err error

		n, err = daemon.FindNetwork(networkName)
		if err == nil {
			networkName = n.Name()
		}
	}

	if ctr.NetworkSettings == nil {
		ctr.NetworkSettings = &network.Settings{}
	}

	if len(endpointsConfig) > 0 {
		if ctr.NetworkSettings.Networks == nil {
			ctr.NetworkSettings.Networks = make(map[string]*network.EndpointSettings)
		}

		for name, epConfig := range endpointsConfig {
			ctr.NetworkSettings.Networks[name] = &network.EndpointSettings{
				EndpointSettings: epConfig,
				// At this point, during container creation, epConfig.MacAddress is the
				// configured value from the API. If there is no configured value, the
				// same field will later be used to store a generated MAC address. So,
				// remember the requested address now.
				DesiredMacAddress: epConfig.MacAddress,
			}
		}
	}

	if ctr.NetworkSettings.Networks == nil {
		ctr.NetworkSettings.Networks = make(map[string]*network.EndpointSettings)
		ctr.NetworkSettings.Networks[networkName] = &network.EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{},
		}
	}

	if !mode.IsUserDefined() {
		return
	}
	// Make sure to internally store the per network endpoint config by network name
	if _, ok := ctr.NetworkSettings.Networks[networkName]; ok {
		return
	}

	if n != nil {
		if nwConfig, ok := ctr.NetworkSettings.Networks[n.ID()]; ok {
			ctr.NetworkSettings.Networks[networkName] = nwConfig
			delete(ctr.NetworkSettings.Networks, n.ID())
			return
		}
	}
}

func (daemon *Daemon) allocateNetwork(ctx context.Context, cfg *config.Config, ctr *container.Container) (retErr error) {
	start := time.Now()

	// An intermediate map is necessary because "connectToNetwork" modifies "container.NetworkSettings.Networks"
	networks := make(map[string]*network.EndpointSettings)
	for n, epConf := range ctr.NetworkSettings.Networks {
		networks[n] = epConf
	}
	for netName, epConf := range networks {
		cleanOperationalData(epConf)
		if err := daemon.connectToNetwork(ctx, cfg, ctr, netName, epConf); err != nil {
			return err
		}
	}

	if _, err := ctr.WriteHostConfig(); err != nil {
		return err
	}
	networkActions.WithValues("allocate").UpdateSince(start)
	return nil
}

// initializeNetworking prepares network configuration for a new container.
// If it creates a new libnetwork.Sandbox it's returned as newSandbox, for
// the caller to Delete() if the container setup fails later in the process.
func (daemon *Daemon) initializeNetworking(ctx context.Context, cfg *config.Config, ctr *container.Container) (newSandbox *libnetwork.Sandbox, retErr error) {
	if daemon.netController == nil || ctr.Config.NetworkDisabled {
		return nil, nil
	}

	// Cleanup any stale sandbox left over due to ungraceful daemon shutdown
	if err := daemon.netController.SandboxDestroy(ctx, ctr.ID); err != nil {
		log.G(ctx).WithError(err).Errorf("failed to cleanup up stale network sandbox for container %s", ctr.ID)
	}

	if ctr.HostConfig.NetworkMode.IsContainer() {
		// we need to get the hosts files from the container to join
		nc, err := daemon.getNetworkedContainer(ctr.ID, ctr.HostConfig.NetworkMode.ConnectedContainer())
		if err != nil {
			return nil, err
		}

		err = daemon.initializeNetworkingPaths(ctr, nc)
		if err != nil {
			return nil, err
		}

		ctr.Config.Hostname = nc.Config.Hostname
		ctr.Config.Domainname = nc.Config.Domainname
		return nil, nil
	}

	if ctr.HostConfig.NetworkMode.IsHost() && ctr.Config.Hostname == "" {
		hn, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		ctr.Config.Hostname = hn
	}

	daemon.updateContainerNetworkSettings(ctr, nil)

	sbOptions, err := buildSandboxOptions(cfg, ctr)
	if err != nil {
		return nil, err
	}
	sb, err := daemon.netController.NewSandbox(ctx, ctr.ID, sbOptions...)
	if err != nil {
		return nil, err
	}

	setNetworkSandbox(ctr, sb)

	defer func() {
		if retErr != nil {
			if err := sb.Delete(ctx); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"error":     err,
					"container": ctr.ID,
				}).Warn("Failed to remove new network sandbox")
			}
		}
	}()

	if err := ctr.BuildHostnameFile(); err != nil {
		return nil, err
	}

	// TODO(robmry) - on Windows, running this after the task has been created does something
	//  strange to the resolver ... addresses are assigned properly (including addresses
	//  specified in the 'run' command), nslookup works, but 'ping' doesn't find the address
	//  of a container. There's no query to our internal resolver from 'ping' (there is from
	//  nslookup), so Windows must have squirreled away the address somewhere else.
	if runtime.GOOS == "windows" {
		if err := daemon.allocateNetwork(ctx, cfg, ctr); err != nil {
			return nil, err
		}
	}

	return newSandbox, nil
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

	if sysctls, ok := epConfig.DriverOpts[netlabel.EndpointSysctls]; ok {
		for _, sysctl := range strings.Split(sysctls, ",") {
			scname := strings.SplitN(sysctl, ".", 5)
			// Allow "ifname" as well as "IFNAME", because the CLI converts to lower case.
			if len(scname) != 5 ||
				(scname[1] != "ipv4" && scname[1] != "ipv6" && scname[1] != "mpls") ||
				(scname[3] != "IFNAME" && scname[3] != "ifname") {
				errs = append(errs,
					fmt.Errorf(
						"unrecognised network interface sysctl '%s'; represent 'net.X.Y.ethN.Z=V' as 'net.X.Y.IFNAME.Z=V', 'X' must be 'ipv4', 'ipv6' or 'mpls'",
						sysctl))
			}
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

func (daemon *Daemon) updateNetworkConfig(ctr *container.Container, n *libnetwork.Network, endpointConfig *networktypes.EndpointSettings) error {
	// Set up DNS names for a user defined network, and for the default 'nat'
	// network on Windows (IsBridge() returns true for nat).
	if containertypes.NetworkMode(n.Name()).IsUserDefined() ||
		(serviceDiscoveryOnDefaultNetwork() && containertypes.NetworkMode(n.Name()).IsBridge()) {
		endpointConfig.DNSNames = buildEndpointDNSNames(ctr, endpointConfig.Aliases)
	}

	if err := validateEndpointSettings(n, n.Name(), endpointConfig); err != nil {
		return errdefs.InvalidParameter(err)
	}

	return daemon.updateNetworkSettings(ctr, n, endpointConfig)
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

func (daemon *Daemon) connectToNetwork(ctx context.Context, cfg *config.Config, ctr *container.Container, idOrName string, endpointConfig *network.EndpointSettings) (retErr error) {
	containerName := strings.TrimPrefix(ctr.Name, "/")
	ctx, span := otel.Tracer("").Start(ctx, "daemon.connectToNetwork", trace.WithAttributes(
		attribute.String("container.ID", ctr.ID),
		attribute.String("container.name", containerName),
		attribute.String("network.idOrName", idOrName)))
	defer span.End()

	start := time.Now()

	if ctr.HostConfig.NetworkMode.IsContainer() {
		return runconfig.ErrConflictSharedNetwork
	}
	if cfg.DisableBridge && containertypes.NetworkMode(idOrName).IsBridge() {
		ctr.Config.NetworkDisabled = true
		return nil
	}
	if endpointConfig == nil {
		endpointConfig = &network.EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{},
		}
	}

	n, nwCfg, err := daemon.findAndAttachNetwork(ctr, idOrName, endpointConfig.EndpointSettings)
	if err != nil {
		return err
	}
	if n == nil {
		return nil
	}
	nwName := n.Name()

	if idOrName != ctr.HostConfig.NetworkMode.NetworkName() {
		if err := daemon.normalizeNetMode(ctr); err != nil {
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

	if err := daemon.updateNetworkConfig(ctr, n, endpointConfig.EndpointSettings); err != nil {
		return err
	}

	sb, err := daemon.netController.GetSandbox(ctr.ID)
	if err != nil {
		return err
	}

	createOptions, err := buildCreateEndpointOptions(ctr, n, endpointConfig, sb, ipAddresses(cfg.DNS))
	if err != nil {
		return err
	}

	ep, err := n.CreateEndpoint(ctx, containerName, createOptions...)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			if err := ep.Delete(context.WithoutCancel(ctx), false); err != nil {
				log.G(ctx).Warnf("Could not rollback container connection to network %s", idOrName)
			}
		}
	}()
	ctr.NetworkSettings.Networks[nwName] = endpointConfig

	delete(ctr.NetworkSettings.Networks, n.ID())

	if err := daemon.updateEndpointNetworkSettings(cfg, ctr, n, ep); err != nil {
		return err
	}

	if nwName == network.DefaultNetwork {
		if err := daemon.addLegacyLinks(ctx, cfg, ctr, endpointConfig, sb); err != nil {
			return err
		}
	}

	joinOptions, err := buildJoinOptions(ctr.NetworkSettings, n)
	if err != nil {
		return err
	}

	if err := ep.Join(ctx, sb, joinOptions...); err != nil {
		return err
	}

	if !ctr.Managed {
		// add container name/alias to DNS
		if err := daemon.ActivateContainerServiceBinding(ctr.Name); err != nil {
			return fmt.Errorf("Activate container service binding for %s failed: %v", ctr.Name, err)
		}
	}

	if err := updateJoinInfo(ctr.NetworkSettings, n, ep); err != nil {
		return fmt.Errorf("Updating join info failed: %v", err)
	}

	ctr.NetworkSettings.Ports = getPortMapInfo(sb)

	daemon.LogNetworkEventWithAttributes(n, events.ActionConnect, map[string]string{"container": ctr.ID})
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
	return ep.Delete(context.TODO(), true)
}

func (daemon *Daemon) disconnectFromNetwork(ctx context.Context, ctr *container.Container, n *libnetwork.Network, force bool) error {
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
			if sb.ContainerID() == ctr.ID {
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
			ep, err = n.EndpointByName(strings.TrimPrefix(ctr.Name, "/"))
			if err != nil {
				return err
			}
			return ep.Delete(ctx, force)
		}
		return fmt.Errorf("container %s is not connected to network %s", ctr.ID, n.Name())
	}

	if err := ep.Leave(ctx, sbox); err != nil {
		return fmt.Errorf("container %s failed to leave network %s: %v", ctr.ID, n.Name(), err)
	}

	ctr.NetworkSettings.Ports = getPortMapInfo(sbox)

	if err := ep.Delete(ctx, false); err != nil {
		return fmt.Errorf("endpoint delete failed for container %s on network %s: %v", ctr.ID, n.Name(), err)
	}

	delete(ctr.NetworkSettings.Networks, n.Name())

	daemon.tryDetachContainerFromClusterNetwork(n, ctr)

	return nil
}

func (daemon *Daemon) tryDetachContainerFromClusterNetwork(network *libnetwork.Network, ctr *container.Container) {
	if !ctr.Managed && daemon.clusterProvider != nil && network.Dynamic() {
		if err := daemon.clusterProvider.DetachNetwork(network.Name(), ctr.ID); err != nil {
			log.G(context.TODO()).WithError(err).Warn("error detaching from network")
			if err := daemon.clusterProvider.DetachNetwork(network.ID(), ctr.ID); err != nil {
				log.G(context.TODO()).WithError(err).Warn("error detaching from network")
			}
		}
	}
	daemon.LogNetworkEventWithAttributes(network, events.ActionDisconnect, map[string]string{
		"container": ctr.ID,
	})
}

// normalizeNetMode checks whether the network mode references a network by a partial ID. In that case, it replaces the
// partial ID with the full network ID.
// TODO(aker): transform ID into name when the referenced network is one of the predefined.
func (daemon *Daemon) normalizeNetMode(ctr *container.Container) error {
	if ctr.HostConfig.NetworkMode.IsUserDefined() {
		netMode := ctr.HostConfig.NetworkMode.NetworkName()
		nw, err := daemon.FindNetwork(netMode)
		if err != nil {
			return fmt.Errorf("could not find a network matching network mode %s: %w", netMode, err)
		}

		if netMode != nw.ID() && netMode != nw.Name() {
			ctr.HostConfig.NetworkMode = containertypes.NetworkMode(nw.ID())
		}
	}

	return nil
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

func (daemon *Daemon) releaseNetwork(ctx context.Context, ctr *container.Container) {
	ctx = context.WithoutCancel(ctx)

	start := time.Now()
	// If live-restore is enabled, the daemon cleans up dead containers when it starts up. In that case, the
	// netController hasn't been initialized yet, and so we can't proceed.
	if daemon.netController == nil {
		return
	}
	// If the container uses the network namespace of another container, it doesn't own it -- nothing to do here.
	if ctr.HostConfig.NetworkMode.IsContainer() {
		return
	}
	if ctr.NetworkSettings == nil {
		return
	}

	ctr.NetworkSettings.Ports = nil
	sid := ctr.NetworkSettings.SandboxID
	if sid == "" {
		return
	}

	ctr.NetworkSettings.SandboxID = ""
	ctr.NetworkSettings.SandboxKey = ""

	var networks []*libnetwork.Network
	for n, epSettings := range ctr.NetworkSettings.Networks {
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
		log.G(ctx).Warnf("error locating sandbox id %s: %v", sid, err)
		return
	}

	if err := sb.Delete(ctx); err != nil {
		log.G(ctx).Errorf("Error deleting sandbox id %s for container %s: %v", sid, ctr.ID, err)
	}

	for _, nw := range networks {
		daemon.tryDetachContainerFromClusterNetwork(nw, ctr)
	}
	networkActions.WithValues("release").UpdateSince(start)
}

func errRemovalContainer(containerID string) error {
	return fmt.Errorf("Container %s is marked for removal and cannot be connected or disconnected to the network", containerID)
}

// ConnectToNetwork connects a container to a network
func (daemon *Daemon) ConnectToNetwork(ctx context.Context, ctr *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings) error {
	if endpointConfig == nil {
		endpointConfig = &networktypes.EndpointSettings{}
	}
	ctr.Lock()
	defer ctr.Unlock()

	if !ctr.Running {
		if ctr.RemovalInProgress || ctr.Dead {
			return errRemovalContainer(ctr.ID)
		}

		n, err := daemon.FindNetwork(idOrName)
		if err == nil && n != nil {
			if err := daemon.updateNetworkConfig(ctr, n, endpointConfig); err != nil {
				return err
			}
		} else {
			ctr.NetworkSettings.Networks[idOrName] = &network.EndpointSettings{
				EndpointSettings: endpointConfig,
			}
		}
	} else {
		epc := &network.EndpointSettings{
			EndpointSettings: endpointConfig,
		}
		if err := daemon.connectToNetwork(ctx, &daemon.config().Config, ctr, idOrName, epc); err != nil {
			return err
		}
	}

	return ctr.CheckpointTo(ctx, daemon.containersReplica)
}

// DisconnectFromNetwork disconnects container from network n.
func (daemon *Daemon) DisconnectFromNetwork(ctx context.Context, ctr *container.Container, networkName string, force bool) error {
	n, err := daemon.FindNetwork(networkName)
	ctr.Lock()
	defer ctr.Unlock()

	if !ctr.Running || (err != nil && force) {
		if ctr.RemovalInProgress || ctr.Dead {
			return errRemovalContainer(ctr.ID)
		}
		// In case networkName is resolved we will use n.Name()
		// this will cover the case where network id is passed.
		if n != nil {
			networkName = n.Name()
		}
		if _, ok := ctr.NetworkSettings.Networks[networkName]; !ok {
			return fmt.Errorf("container %s is not connected to the network %s", ctr.ID, networkName)
		}
		delete(ctr.NetworkSettings.Networks, networkName)
	} else if err == nil {
		if ctr.HostConfig.NetworkMode.IsHost() && containertypes.NetworkMode(n.Type()).IsHost() {
			return runconfig.ErrConflictHostNetwork
		}

		if err := daemon.disconnectFromNetwork(ctx, ctr, n, false); err != nil {
			return err
		}
	} else {
		return err
	}

	if err := ctr.CheckpointTo(ctx, daemon.containersReplica); err != nil {
		return err
	}

	if n != nil {
		daemon.LogNetworkEventWithAttributes(n, events.ActionDisconnect, map[string]string{
			"container": ctr.ID,
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
func setNetworkSandbox(ctr *container.Container, sb *libnetwork.Sandbox) {
	ctr.NetworkSettings.SandboxID = sb.ID()
	ctr.NetworkSettings.SandboxKey = sb.Key()
}
