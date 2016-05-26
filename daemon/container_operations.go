package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/network"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	containertypes "github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
)

var (
	// ErrRootFSReadOnly is returned when a container
	// rootfs is marked readonly.
	ErrRootFSReadOnly = errors.New("container rootfs is marked read-only")
	getPortMapInfo    = container.GetSandboxPortMapInfo
)

func (daemon *Daemon) buildSandboxOptions(container *container.Container, n libnetwork.Network) ([]libnetwork.SandboxOption, error) {
	var (
		sboxOptions []libnetwork.SandboxOption
		err         error
		dns         []string
		dnsSearch   []string
		dnsOptions  []string
		bindings    = make(nat.PortMap)
		pbList      []types.PortBinding
		exposeList  []types.TransportPort
	)

	defaultNetName := runconfig.DefaultDaemonNetworkMode().NetworkName()
	sboxOptions = append(sboxOptions, libnetwork.OptionHostname(container.Config.Hostname),
		libnetwork.OptionDomainname(container.Config.Domainname))

	if container.HostConfig.NetworkMode.IsHost() {
		sboxOptions = append(sboxOptions, libnetwork.OptionUseDefaultSandbox())
		if len(container.HostConfig.ExtraHosts) == 0 {
			sboxOptions = append(sboxOptions, libnetwork.OptionOriginHostsPath("/etc/hosts"))
		}
		if len(container.HostConfig.DNS) == 0 && len(daemon.configStore.DNS) == 0 &&
			len(container.HostConfig.DNSSearch) == 0 && len(daemon.configStore.DNSSearch) == 0 &&
			len(container.HostConfig.DNSOptions) == 0 && len(daemon.configStore.DNSOptions) == 0 {
			sboxOptions = append(sboxOptions, libnetwork.OptionOriginResolvConfPath("/etc/resolv.conf"))
		}
	} else {
		// OptionUseExternalKey is mandatory for userns support.
		// But optional for non-userns support
		sboxOptions = append(sboxOptions, libnetwork.OptionUseExternalKey())
	}

	container.HostsPath, err = container.GetRootResourcePath("hosts")
	if err != nil {
		return nil, err
	}
	sboxOptions = append(sboxOptions, libnetwork.OptionHostsPath(container.HostsPath))

	container.ResolvConfPath, err = container.GetRootResourcePath("resolv.conf")
	if err != nil {
		return nil, err
	}
	sboxOptions = append(sboxOptions, libnetwork.OptionResolvConfPath(container.ResolvConfPath))

	if len(container.HostConfig.DNS) > 0 {
		dns = container.HostConfig.DNS
	} else if len(daemon.configStore.DNS) > 0 {
		dns = daemon.configStore.DNS
	}

	for _, d := range dns {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNS(d))
	}

	if len(container.HostConfig.DNSSearch) > 0 {
		dnsSearch = container.HostConfig.DNSSearch
	} else if len(daemon.configStore.DNSSearch) > 0 {
		dnsSearch = daemon.configStore.DNSSearch
	}

	for _, ds := range dnsSearch {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSSearch(ds))
	}

	if len(container.HostConfig.DNSOptions) > 0 {
		dnsOptions = container.HostConfig.DNSOptions
	} else if len(daemon.configStore.DNSOptions) > 0 {
		dnsOptions = daemon.configStore.DNSOptions
	}

	for _, ds := range dnsOptions {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSOptions(ds))
	}

	if container.NetworkSettings.SecondaryIPAddresses != nil {
		name := container.Config.Hostname
		if container.Config.Domainname != "" {
			name = name + "." + container.Config.Domainname
		}

		for _, a := range container.NetworkSettings.SecondaryIPAddresses {
			sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(name, a.Addr))
		}
	}

	for _, extraHost := range container.HostConfig.ExtraHosts {
		// allow IPv6 addresses in extra hosts; only split on first ":"
		parts := strings.SplitN(extraHost, ":", 2)
		sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(parts[0], parts[1]))
	}

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

	portSpecs := container.Config.ExposedPorts
	ports := make([]nat.Port, len(portSpecs))
	var i int
	for p := range portSpecs {
		ports[i] = p
		i++
	}
	nat.SortPortMap(ports, bindings)
	for _, port := range ports {
		expose := types.TransportPort{}
		expose.Proto = types.ParseProtocol(port.Proto())
		expose.Port = uint16(port.Int())
		exposeList = append(exposeList, expose)

		pb := types.PortBinding{Port: expose.Port, Proto: expose.Proto}
		binding := bindings[port]
		for i := 0; i < len(binding); i++ {
			pbCopy := pb.GetCopy()
			newP, err := nat.NewPort(nat.SplitProtoPort(binding[i].HostPort))
			var portStart, portEnd int
			if err == nil {
				portStart, portEnd, err = newP.Range()
			}
			if err != nil {
				return nil, fmt.Errorf("Error parsing HostPort value(%s):%v", binding[i].HostPort, err)
			}
			pbCopy.HostPort = uint16(portStart)
			pbCopy.HostPortEnd = uint16(portEnd)
			pbCopy.HostIP = net.ParseIP(binding[i].HostIP)
			pbList = append(pbList, pbCopy)
		}

		if container.HostConfig.PublishAllPorts && len(binding) == 0 {
			pbList = append(pbList, pb)
		}
	}

	sboxOptions = append(sboxOptions,
		libnetwork.OptionPortMapping(pbList),
		libnetwork.OptionExposedPorts(exposeList))

	// Legacy Link feature is supported only for the default bridge network.
	// return if this call to build join options is not for default bridge network
	if n.Name() != defaultNetName {
		return sboxOptions, nil
	}

	ep, _ := container.GetEndpointInNetwork(n)
	if ep == nil {
		return sboxOptions, nil
	}

	var childEndpoints, parentEndpoints []string

	children := daemon.children(container)
	for linkAlias, child := range children {
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
		sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(aliasList, child.NetworkSettings.Networks[defaultNetName].IPAddress))
		cEndpoint, _ := child.GetEndpointInNetwork(n)
		if cEndpoint != nil && cEndpoint.ID() != "" {
			childEndpoints = append(childEndpoints, cEndpoint.ID())
		}
	}

	bridgeSettings := container.NetworkSettings.Networks[defaultNetName]
	for alias, parent := range daemon.parents(container) {
		if daemon.configStore.DisableBridge || !container.HostConfig.NetworkMode.IsPrivate() {
			continue
		}

		_, alias = path.Split(alias)
		logrus.Debugf("Update /etc/hosts of %s for alias %s with ip %s", parent.ID, alias, bridgeSettings.IPAddress)
		sboxOptions = append(sboxOptions, libnetwork.OptionParentUpdate(
			parent.ID,
			alias,
			bridgeSettings.IPAddress,
		))
		if ep.ID() != "" {
			parentEndpoints = append(parentEndpoints, ep.ID())
		}
	}

	linkOptions := options.Generic{
		netlabel.GenericData: options.Generic{
			"ParentEndpoints": parentEndpoints,
			"ChildEndpoints":  childEndpoints,
		},
	}

	sboxOptions = append(sboxOptions, libnetwork.OptionGeneric(linkOptions))
	return sboxOptions, nil
}

func (daemon *Daemon) updateNetworkSettings(container *container.Container, n libnetwork.Network) error {
	if container.NetworkSettings == nil {
		container.NetworkSettings = &network.Settings{Networks: make(map[string]*networktypes.EndpointSettings)}
	}

	if !container.HostConfig.NetworkMode.IsHost() && containertypes.NetworkMode(n.Type()).IsHost() {
		return runconfig.ErrConflictHostNetwork
	}

	for s := range container.NetworkSettings.Networks {
		sn, err := daemon.FindNetwork(s)
		if err != nil {
			continue
		}

		if sn.Name() == n.Name() {
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

	if _, ok := container.NetworkSettings.Networks[n.Name()]; !ok {
		container.NetworkSettings.Networks[n.Name()] = new(networktypes.EndpointSettings)
	}

	return nil
}

func (daemon *Daemon) updateEndpointNetworkSettings(container *container.Container, n libnetwork.Network, ep libnetwork.Endpoint) error {
	if err := container.BuildEndpointInfo(n, ep); err != nil {
		return err
	}

	if container.HostConfig.NetworkMode == runconfig.DefaultDaemonNetworkMode() {
		container.NetworkSettings.Bridge = daemon.configStore.bridgeConfig.Iface
	}

	return nil
}

// UpdateNetwork is used to update the container's network (e.g. when linked containers
// get removed/unlinked).
func (daemon *Daemon) updateNetwork(container *container.Container) error {
	ctrl := daemon.netController
	sid := container.NetworkSettings.SandboxID

	sb, err := ctrl.SandboxByID(sid)
	if err != nil {
		return fmt.Errorf("error locating sandbox id %s: %v", sid, err)
	}

	// Find if container is connected to the default bridge network
	var n libnetwork.Network
	for name := range container.NetworkSettings.Networks {
		sn, err := daemon.FindNetwork(name)
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

	options, err := daemon.buildSandboxOptions(container, n)
	if err != nil {
		return fmt.Errorf("Update network failed: %v", err)
	}

	if err := sb.Refresh(options...); err != nil {
		return fmt.Errorf("Update network failed: Failure in refresh sandbox %s: %v", sid, err)
	}

	return nil
}

// updateContainerNetworkSettings update the network settings
func (daemon *Daemon) updateContainerNetworkSettings(container *container.Container, endpointsConfig map[string]*networktypes.EndpointSettings) error {
	var (
		n   libnetwork.Network
		err error
	)

	mode := container.HostConfig.NetworkMode
	if container.Config.NetworkDisabled || mode.IsContainer() {
		return nil
	}

	networkName := mode.NetworkName()
	if mode.IsDefault() {
		networkName = daemon.netController.Config().Daemon.DefaultNetwork
	}
	if mode.IsUserDefined() {
		n, err = daemon.FindNetwork(networkName)
		if err != nil {
			return err
		}
		networkName = n.Name()
	}
	if container.NetworkSettings == nil {
		container.NetworkSettings = &network.Settings{}
	}
	if len(endpointsConfig) > 0 {
		container.NetworkSettings.Networks = endpointsConfig
	}
	if container.NetworkSettings.Networks == nil {
		container.NetworkSettings.Networks = make(map[string]*networktypes.EndpointSettings)
		container.NetworkSettings.Networks[networkName] = new(networktypes.EndpointSettings)
	}
	if !mode.IsUserDefined() {
		return nil
	}
	// Make sure to internally store the per network endpoint config by network name
	if _, ok := container.NetworkSettings.Networks[networkName]; ok {
		return nil
	}
	if nwConfig, ok := container.NetworkSettings.Networks[n.ID()]; ok {
		container.NetworkSettings.Networks[networkName] = nwConfig
		delete(container.NetworkSettings.Networks, n.ID())
		return nil
	}

	return nil
}

func (daemon *Daemon) allocateNetwork(container *container.Container) error {
	controller := daemon.netController

	if daemon.netController == nil {
		return nil
	}

	// Cleanup any stale sandbox left over due to ungraceful daemon shutdown
	if err := controller.SandboxDestroy(container.ID); err != nil {
		logrus.Errorf("failed to cleanup up stale network sandbox for container %s", container.ID)
	}

	updateSettings := false
	if len(container.NetworkSettings.Networks) == 0 {
		if container.Config.NetworkDisabled || container.HostConfig.NetworkMode.IsContainer() {
			return nil
		}

		err := daemon.updateContainerNetworkSettings(container, nil)
		if err != nil {
			return err
		}
		updateSettings = true
	}

	for n, nConf := range container.NetworkSettings.Networks {
		if err := daemon.connectToNetwork(container, n, nConf, updateSettings); err != nil {
			return err
		}
	}

	return container.WriteHostConfig()
}

func (daemon *Daemon) getNetworkSandbox(container *container.Container) libnetwork.Sandbox {
	var sb libnetwork.Sandbox
	daemon.netController.WalkSandboxes(func(s libnetwork.Sandbox) bool {
		if s.ContainerID() == container.ID {
			sb = s
			return true
		}
		return false
	})
	return sb
}

// hasUserDefinedIPAddress returns whether the passed endpoint configuration contains IP address configuration
func hasUserDefinedIPAddress(epConfig *networktypes.EndpointSettings) bool {
	return epConfig != nil && epConfig.IPAMConfig != nil && (len(epConfig.IPAMConfig.IPv4Address) > 0 || len(epConfig.IPAMConfig.IPv6Address) > 0)
}

// User specified ip address is acceptable only for networks with user specified subnets.
func validateNetworkingConfig(n libnetwork.Network, epConfig *networktypes.EndpointSettings) error {
	if n == nil || epConfig == nil {
		return nil
	}
	if !hasUserDefinedIPAddress(epConfig) {
		return nil
	}
	_, _, nwIPv4Configs, nwIPv6Configs := n.Info().IpamConfig()
	for _, s := range []struct {
		ipConfigured  bool
		subnetConfigs []*libnetwork.IpamConf
	}{
		{
			ipConfigured:  len(epConfig.IPAMConfig.IPv4Address) > 0,
			subnetConfigs: nwIPv4Configs,
		},
		{
			ipConfigured:  len(epConfig.IPAMConfig.IPv6Address) > 0,
			subnetConfigs: nwIPv6Configs,
		},
	} {
		if s.ipConfigured {
			foundSubnet := false
			for _, cfg := range s.subnetConfigs {
				if len(cfg.PreferredPool) > 0 {
					foundSubnet = true
					break
				}
			}
			if !foundSubnet {
				return runconfig.ErrUnsupportedNetworkNoSubnetAndIP
			}
		}
	}

	return nil
}

// cleanOperationalData resets the operational data from the passed endpoint settings
func cleanOperationalData(es *networktypes.EndpointSettings) {
	es.EndpointID = ""
	es.Gateway = ""
	es.IPAddress = ""
	es.IPPrefixLen = 0
	es.IPv6Gateway = ""
	es.GlobalIPv6Address = ""
	es.GlobalIPv6PrefixLen = 0
	es.MacAddress = ""
}

func (daemon *Daemon) updateNetworkConfig(container *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings, updateSettings bool) (libnetwork.Network, error) {
	if container.HostConfig.NetworkMode.IsContainer() {
		return nil, runconfig.ErrConflictSharedNetwork
	}

	if containertypes.NetworkMode(idOrName).IsBridge() &&
		daemon.configStore.DisableBridge {
		container.Config.NetworkDisabled = true
		return nil, nil
	}

	if !containertypes.NetworkMode(idOrName).IsUserDefined() {
		if hasUserDefinedIPAddress(endpointConfig) {
			return nil, runconfig.ErrUnsupportedNetworkAndIP
		}
		if endpointConfig != nil && len(endpointConfig.Aliases) > 0 {
			return nil, runconfig.ErrUnsupportedNetworkAndAlias
		}
	} else {
		addShortID := true
		shortID := stringid.TruncateID(container.ID)
		for _, alias := range endpointConfig.Aliases {
			if alias == shortID {
				addShortID = false
				break
			}
		}
		if addShortID {
			endpointConfig.Aliases = append(endpointConfig.Aliases, shortID)
		}
	}

	n, err := daemon.FindNetwork(idOrName)
	if err != nil {
		return nil, err
	}

	if err := validateNetworkingConfig(n, endpointConfig); err != nil {
		return nil, err
	}

	if updateSettings {
		if err := daemon.updateNetworkSettings(container, n); err != nil {
			return nil, err
		}
	}
	return n, nil
}

func (daemon *Daemon) connectToNetwork(container *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings, updateSettings bool) (err error) {
	if endpointConfig == nil {
		endpointConfig = &networktypes.EndpointSettings{}
	}
	n, err := daemon.updateNetworkConfig(container, idOrName, endpointConfig, updateSettings)
	if err != nil {
		return err
	}
	if n == nil {
		return nil
	}

	controller := daemon.netController

	sb := daemon.getNetworkSandbox(container)
	createOptions, err := container.BuildCreateEndpointOptions(n, endpointConfig, sb)
	if err != nil {
		return err
	}

	endpointName := strings.TrimPrefix(container.Name, "/")
	ep, err := n.CreateEndpoint(endpointName, createOptions...)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if e := ep.Delete(false); e != nil {
				logrus.Warnf("Could not rollback container connection to network %s", idOrName)
			}
		}
	}()
	container.NetworkSettings.Networks[n.Name()] = endpointConfig

	if err := daemon.updateEndpointNetworkSettings(container, n, ep); err != nil {
		return err
	}

	if sb == nil {
		options, err := daemon.buildSandboxOptions(container, n)
		if err != nil {
			return err
		}
		sb, err = controller.NewSandbox(container.ID, options...)
		if err != nil {
			return err
		}

		container.UpdateSandboxNetworkSettings(sb)
	}

	joinOptions, err := container.BuildJoinOptions(n)
	if err != nil {
		return err
	}

	if err := ep.Join(sb, joinOptions...); err != nil {
		return err
	}

	if err := container.UpdateJoinInfo(n, ep); err != nil {
		return fmt.Errorf("Updating join info failed: %v", err)
	}

	container.NetworkSettings.Ports = getPortMapInfo(sb)

	daemon.LogNetworkEventWithAttributes(n, "connect", map[string]string{"container": container.ID})
	return nil
}

// ForceEndpointDelete deletes an endpoing from a network forcefully
func (daemon *Daemon) ForceEndpointDelete(name string, n libnetwork.Network) error {
	ep, err := n.EndpointByName(name)
	if err != nil {
		return err
	}
	return ep.Delete(true)
}

func disconnectFromNetwork(container *container.Container, n libnetwork.Network, force bool) error {
	var (
		ep   libnetwork.Endpoint
		sbox libnetwork.Sandbox
	)

	s := func(current libnetwork.Endpoint) bool {
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
	}
	n.WalkEndpoints(s)

	if ep == nil && force {
		epName := strings.TrimPrefix(container.Name, "/")
		ep, err := n.EndpointByName(epName)
		if err != nil {
			return err
		}
		return ep.Delete(force)
	}

	if ep == nil {
		return fmt.Errorf("container %s is not connected to the network", container.ID)
	}

	if err := ep.Leave(sbox); err != nil {
		return fmt.Errorf("container %s failed to leave network %s: %v", container.ID, n.Name(), err)
	}

	container.NetworkSettings.Ports = getPortMapInfo(sbox)

	if err := ep.Delete(false); err != nil {
		return fmt.Errorf("endpoint delete failed for container %s on network %s: %v", container.ID, n.Name(), err)
	}

	delete(container.NetworkSettings.Networks, n.Name())
	return nil
}

func (daemon *Daemon) initializeNetworking(container *container.Container) error {
	var err error

	if container.HostConfig.NetworkMode.IsContainer() {
		// we need to get the hosts files from the container to join
		nc, err := daemon.getNetworkedContainer(container.ID, container.HostConfig.NetworkMode.ConnectedContainer())
		if err != nil {
			return err
		}
		container.HostnamePath = nc.HostnamePath
		container.HostsPath = nc.HostsPath
		container.ResolvConfPath = nc.ResolvConfPath
		container.Config.Hostname = nc.Config.Hostname
		container.Config.Domainname = nc.Config.Domainname
		return nil
	}

	if container.HostConfig.NetworkMode.IsHost() {
		container.Config.Hostname, err = os.Hostname()
		if err != nil {
			return err
		}
	}

	if err := daemon.allocateNetwork(container); err != nil {
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
		err := fmt.Errorf("cannot join network of a non running container: %s", connectedContainerID)
		return nil, derr.NewRequestConflictError(err)
	}
	if nc.IsRestarting() {
		return nil, errContainerIsRestarting(connectedContainerID)
	}
	return nc, nil
}

func (daemon *Daemon) releaseNetwork(container *container.Container) {
	if container.HostConfig.NetworkMode.IsContainer() || container.Config.NetworkDisabled {
		return
	}

	sid := container.NetworkSettings.SandboxID
	settings := container.NetworkSettings.Networks
	container.NetworkSettings.Ports = nil

	if sid == "" || len(settings) == 0 {
		return
	}

	var networks []libnetwork.Network
	for n, epSettings := range settings {
		if nw, err := daemon.FindNetwork(n); err == nil {
			networks = append(networks, nw)
		}
		cleanOperationalData(epSettings)
	}

	sb, err := daemon.netController.SandboxByID(sid)
	if err != nil {
		logrus.Warnf("error locating sandbox id %s: %v", sid, err)
		return
	}

	if err := sb.Delete(); err != nil {
		logrus.Errorf("Error deleting sandbox id %s for container %s: %v", sid, container.ID, err)
	}

	attributes := map[string]string{
		"container": container.ID,
	}
	for _, nw := range networks {
		daemon.LogNetworkEventWithAttributes(nw, "disconnect", attributes)
	}
}
