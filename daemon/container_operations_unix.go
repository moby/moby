// +build linux freebsd

package daemon

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/links"
	"github.com/docker/docker/daemon/network"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	containertypes "github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/docker/go-units"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/label"
)

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	var env []string
	children := daemon.children(container)

	bridgeSettings := container.NetworkSettings.Networks["bridge"]
	if bridgeSettings == nil {
		return nil, nil
	}

	for linkAlias, child := range children {
		if !child.IsRunning() {
			return nil, derr.ErrorCodeLinkNotRunning.WithArgs(child.Name, linkAlias)
		}

		childBridgeSettings := child.NetworkSettings.Networks["bridge"]
		if childBridgeSettings == nil {
			return nil, fmt.Errorf("container %s not attached to default bridge network", child.ID)
		}

		link := links.NewLink(
			bridgeSettings.IPAddress,
			childBridgeSettings.IPAddress,
			linkAlias,
			child.Config.Env,
			child.Config.ExposedPorts,
		)

		for _, envVar := range link.ToEnv() {
			env = append(env, envVar)
		}
	}

	return env, nil
}

func (daemon *Daemon) populateCommand(c *container.Container, env []string) error {
	var en *execdriver.Network
	if !c.Config.NetworkDisabled {
		en = &execdriver.Network{}
		if !daemon.execDriver.SupportsHooks() || c.HostConfig.NetworkMode.IsHost() {
			en.NamespacePath = c.NetworkSettings.SandboxKey
		}

		if c.HostConfig.NetworkMode.IsContainer() {
			nc, err := daemon.getNetworkedContainer(c.ID, c.HostConfig.NetworkMode.ConnectedContainer())
			if err != nil {
				return err
			}
			en.ContainerID = nc.ID
		}
	}

	ipc := &execdriver.Ipc{}
	var err error
	c.ShmPath, err = c.ShmResourcePath()
	if err != nil {
		return err
	}

	if c.HostConfig.IpcMode.IsContainer() {
		ic, err := daemon.getIpcContainer(c)
		if err != nil {
			return err
		}
		ipc.ContainerID = ic.ID
		c.ShmPath = ic.ShmPath
	} else {
		ipc.HostIpc = c.HostConfig.IpcMode.IsHost()
		if ipc.HostIpc {
			if _, err := os.Stat("/dev/shm"); err != nil {
				return fmt.Errorf("/dev/shm is not mounted, but must be for --ipc=host")
			}
			c.ShmPath = "/dev/shm"
		}
	}

	pid := &execdriver.Pid{}
	pid.HostPid = c.HostConfig.PidMode.IsHost()

	uts := &execdriver.UTS{
		HostUTS: c.HostConfig.UTSMode.IsHost(),
	}

	// Build lists of devices allowed and created within the container.
	var userSpecifiedDevices []*configs.Device
	for _, deviceMapping := range c.HostConfig.Devices {
		devs, err := getDevicesFromPath(deviceMapping)
		if err != nil {
			return err
		}

		userSpecifiedDevices = append(userSpecifiedDevices, devs...)
	}

	allowedDevices := mergeDevices(configs.DefaultAllowedDevices, userSpecifiedDevices)

	autoCreatedDevices := mergeDevices(configs.DefaultAutoCreatedDevices, userSpecifiedDevices)

	var rlimits []*units.Rlimit
	ulimits := c.HostConfig.Ulimits

	// Merge ulimits with daemon defaults
	ulIdx := make(map[string]*units.Ulimit)
	for _, ul := range ulimits {
		ulIdx[ul.Name] = ul
	}
	for name, ul := range daemon.configStore.Ulimits {
		if _, exists := ulIdx[name]; !exists {
			ulimits = append(ulimits, ul)
		}
	}

	weightDevices, err := getBlkioWeightDevices(c.HostConfig)
	if err != nil {
		return err
	}

	readBpsDevice, err := getBlkioReadBpsDevices(c.HostConfig)
	if err != nil {
		return err
	}

	writeBpsDevice, err := getBlkioWriteBpsDevices(c.HostConfig)
	if err != nil {
		return err
	}

	readIOpsDevice, err := getBlkioReadIOpsDevices(c.HostConfig)
	if err != nil {
		return err
	}

	writeIOpsDevice, err := getBlkioWriteIOpsDevices(c.HostConfig)
	if err != nil {
		return err
	}

	for _, limit := range ulimits {
		rl, err := limit.GetRlimit()
		if err != nil {
			return err
		}
		rlimits = append(rlimits, rl)
	}

	resources := &execdriver.Resources{
		CommonResources: execdriver.CommonResources{
			Memory:            c.HostConfig.Memory,
			MemoryReservation: c.HostConfig.MemoryReservation,
			CPUShares:         c.HostConfig.CPUShares,
			BlkioWeight:       c.HostConfig.BlkioWeight,
		},
		MemorySwap:                   c.HostConfig.MemorySwap,
		KernelMemory:                 c.HostConfig.KernelMemory,
		CpusetCpus:                   c.HostConfig.CpusetCpus,
		CpusetMems:                   c.HostConfig.CpusetMems,
		CPUPeriod:                    c.HostConfig.CPUPeriod,
		CPUQuota:                     c.HostConfig.CPUQuota,
		Rlimits:                      rlimits,
		BlkioWeightDevice:            weightDevices,
		BlkioThrottleReadBpsDevice:   readBpsDevice,
		BlkioThrottleWriteBpsDevice:  writeBpsDevice,
		BlkioThrottleReadIOpsDevice:  readIOpsDevice,
		BlkioThrottleWriteIOpsDevice: writeIOpsDevice,
		MemorySwappiness:             -1,
	}

	if c.HostConfig.OomKillDisable != nil {
		resources.OomKillDisable = *c.HostConfig.OomKillDisable
	}
	if c.HostConfig.MemorySwappiness != nil {
		resources.MemorySwappiness = *c.HostConfig.MemorySwappiness
	}

	processConfig := execdriver.ProcessConfig{
		CommonProcessConfig: execdriver.CommonProcessConfig{
			Entrypoint: c.Path,
			Arguments:  c.Args,
			Tty:        c.Config.Tty,
		},
		Privileged: c.HostConfig.Privileged,
		User:       c.Config.User,
	}

	processConfig.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	processConfig.Env = env

	remappedRoot := &execdriver.User{}
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	if rootUID != 0 {
		remappedRoot.UID = rootUID
		remappedRoot.GID = rootGID
	}
	uidMap, gidMap := daemon.GetUIDGIDMaps()

	if !daemon.seccompEnabled {
		if c.SeccompProfile != "" && c.SeccompProfile != "unconfined" {
			return fmt.Errorf("Seccomp is not enabled in your kernel, cannot run a custom seccomp profile.")
		}
		logrus.Warn("Seccomp is not enabled in your kernel, running container without default profile.")
		c.SeccompProfile = "unconfined"
	}

	defaultCgroupParent := "/docker"
	if daemon.configStore.CgroupParent != "" {
		defaultCgroupParent = daemon.configStore.CgroupParent
	} else if daemon.usingSystemd() {
		defaultCgroupParent = "system.slice"
	}
	c.Command = &execdriver.Command{
		CommonCommand: execdriver.CommonCommand{
			ID:            c.ID,
			MountLabel:    c.GetMountLabel(),
			Network:       en,
			ProcessConfig: processConfig,
			ProcessLabel:  c.GetProcessLabel(),
			Rootfs:        c.BaseFS,
			Resources:     resources,
			WorkingDir:    c.Config.WorkingDir,
		},
		AllowedDevices:     allowedDevices,
		AppArmorProfile:    c.AppArmorProfile,
		AutoCreatedDevices: autoCreatedDevices,
		CapAdd:             c.HostConfig.CapAdd,
		CapDrop:            c.HostConfig.CapDrop,
		CgroupParent:       defaultCgroupParent,
		GIDMapping:         gidMap,
		GroupAdd:           c.HostConfig.GroupAdd,
		Ipc:                ipc,
		OomScoreAdj:        c.HostConfig.OomScoreAdj,
		Pid:                pid,
		ReadonlyRootfs:     c.HostConfig.ReadonlyRootfs,
		RemappedRoot:       remappedRoot,
		SeccompProfile:     c.SeccompProfile,
		UIDMapping:         uidMap,
		UTS:                uts,
	}
	if c.HostConfig.CgroupParent != "" {
		c.Command.CgroupParent = c.HostConfig.CgroupParent
	}

	return nil
}

// getSize returns the real size & virtual size of the container.
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
	var (
		sizeRw, sizeRootfs int64
		err                error
	)

	if err := daemon.Mount(container); err != nil {
		logrus.Errorf("Failed to compute size of container rootfs %s: %s", container.ID, err)
		return sizeRw, sizeRootfs
	}
	defer daemon.Unmount(container)

	sizeRw, err = container.RWLayer.Size()
	if err != nil {
		logrus.Errorf("Driver %s couldn't return diff size of container %s: %s",
			daemon.GraphDriverName(), container.ID, err)
		// FIXME: GetSize should return an error. Not changing it now in case
		// there is a side-effect.
		sizeRw = -1
	}

	if parent := container.RWLayer.Parent(); parent != nil {
		sizeRootfs, err = parent.Size()
		if err != nil {
			sizeRootfs = -1
		} else if sizeRw != -1 {
			sizeRootfs += sizeRw
		}
	}
	return sizeRw, sizeRootfs
}

func (daemon *Daemon) buildSandboxOptions(container *container.Container, n libnetwork.Network) ([]libnetwork.SandboxOption, error) {
	var (
		sboxOptions []libnetwork.SandboxOption
		err         error
		dns         []string
		dnsSearch   []string
		dnsOptions  []string
	)

	sboxOptions = append(sboxOptions, libnetwork.OptionHostname(container.Config.Hostname),
		libnetwork.OptionDomainname(container.Config.Domainname))

	if container.HostConfig.NetworkMode.IsHost() {
		sboxOptions = append(sboxOptions, libnetwork.OptionUseDefaultSandbox())
		sboxOptions = append(sboxOptions, libnetwork.OptionOriginHostsPath("/etc/hosts"))
		sboxOptions = append(sboxOptions, libnetwork.OptionOriginResolvConfPath("/etc/resolv.conf"))
	} else if daemon.execDriver.SupportsHooks() {
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

	// Link feature is supported only for the default bridge network.
	// return if this call to build join options is not for default bridge network
	if n.Name() != "bridge" {
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
		sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(aliasList, child.NetworkSettings.Networks["bridge"].IPAddress))
		cEndpoint, _ := child.GetEndpointInNetwork(n)
		if cEndpoint != nil && cEndpoint.ID() != "" {
			childEndpoints = append(childEndpoints, cEndpoint.ID())
		}
	}

	bridgeSettings := container.NetworkSettings.Networks["bridge"]
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

	if container.HostConfig.NetworkMode == containertypes.NetworkMode("bridge") {
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
		return derr.ErrorCodeNoSandbox.WithArgs(sid, err)
	}

	// Find if container is connected to the default bridge network
	var n libnetwork.Network
	for name := range container.NetworkSettings.Networks {
		sn, err := daemon.FindNetwork(name)
		if err != nil {
			continue
		}
		if sn.Name() == "bridge" {
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
		return derr.ErrorCodeNetworkUpdate.WithArgs(err)
	}

	if err := sb.Refresh(options...); err != nil {
		return derr.ErrorCodeNetworkRefresh.WithArgs(sid, err)
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

// ConnectToNetwork connects a container to a network
func (daemon *Daemon) ConnectToNetwork(container *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings) error {
	if !container.Running {
		if container.RemovalInProgress || container.Dead {
			return derr.ErrorCodeRemovalContainer.WithArgs(container.ID)
		}
		if _, err := daemon.updateNetworkConfig(container, idOrName, endpointConfig, true); err != nil {
			return err
		}
		if endpointConfig != nil {
			container.NetworkSettings.Networks[idOrName] = endpointConfig
		}
	} else {
		if err := daemon.connectToNetwork(container, idOrName, endpointConfig, true); err != nil {
			return err
		}
	}
	if err := container.ToDiskLocking(); err != nil {
		return fmt.Errorf("Error saving container to disk: %v", err)
	}
	return nil
}

func (daemon *Daemon) connectToNetwork(container *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings, updateSettings bool) (err error) {
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

	if endpointConfig != nil {
		container.NetworkSettings.Networks[n.Name()] = endpointConfig
	}

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
		return derr.ErrorCodeJoinInfo.WithArgs(err)
	}

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

// DisconnectFromNetwork disconnects container from network n.
func (daemon *Daemon) DisconnectFromNetwork(container *container.Container, n libnetwork.Network, force bool) error {
	if container.HostConfig.NetworkMode.IsHost() && containertypes.NetworkMode(n.Type()).IsHost() {
		return runconfig.ErrConflictHostNetwork
	}
	if !container.Running {
		if container.RemovalInProgress || container.Dead {
			return derr.ErrorCodeRemovalContainer.WithArgs(container.ID)
		}
		if _, ok := container.NetworkSettings.Networks[n.Name()]; ok {
			delete(container.NetworkSettings.Networks, n.Name())
		} else {
			return fmt.Errorf("container %s is not connected to the network %s", container.ID, n.Name())
		}
	} else {
		if err := disconnectFromNetwork(container, n, false); err != nil {
			return err
		}
	}

	if err := container.ToDiskLocking(); err != nil {
		return fmt.Errorf("Error saving container to disk: %v", err)
	}

	attributes := map[string]string{
		"container": container.ID,
	}
	daemon.LogNetworkEventWithAttributes(n, "disconnect", attributes)
	return nil
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

		parts := strings.SplitN(container.Config.Hostname, ".", 2)
		if len(parts) > 1 {
			container.Config.Hostname = parts[0]
			container.Config.Domainname = parts[1]
		}

	}

	if err := daemon.allocateNetwork(container); err != nil {
		return err
	}

	return container.BuildHostnameFile()
}

// called from the libcontainer pre-start hook to set the network
// namespace configuration linkage to the libnetwork "sandbox" entity
func (daemon *Daemon) setNetworkNamespaceKey(containerID string, pid int) error {
	path := fmt.Sprintf("/proc/%d/ns/net", pid)
	var sandbox libnetwork.Sandbox
	search := libnetwork.SandboxContainerWalker(&sandbox, containerID)
	daemon.netController.WalkSandboxes(search)
	if sandbox == nil {
		return derr.ErrorCodeNoSandbox.WithArgs(containerID, "no sandbox found")
	}

	return sandbox.SetKey(path)
}

func (daemon *Daemon) getIpcContainer(container *container.Container) (*container.Container, error) {
	containerID := container.HostConfig.IpcMode.Container()
	c, err := daemon.GetContainer(containerID)
	if err != nil {
		return nil, err
	}
	if !c.IsRunning() {
		return nil, derr.ErrorCodeIPCRunning.WithArgs(containerID)
	}
	if c.IsRestarting() {
		return nil, derr.ErrorCodeContainerRestarting.WithArgs(containerID)
	}
	return c, nil
}

func (daemon *Daemon) getNetworkedContainer(containerID, connectedContainerID string) (*container.Container, error) {
	nc, err := daemon.GetContainer(connectedContainerID)
	if err != nil {
		return nil, err
	}
	if containerID == nc.ID {
		return nil, derr.ErrorCodeJoinSelf
	}
	if !nc.IsRunning() {
		return nil, derr.ErrorCodeJoinRunning.WithArgs(connectedContainerID)
	}
	if nc.IsRestarting() {
		return nil, derr.ErrorCodeContainerRestarting.WithArgs(connectedContainerID)
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
		logrus.Errorf("error locating sandbox id %s: %v", sid, err)
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

func (daemon *Daemon) setupIpcDirs(c *container.Container) error {
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	if !c.HasMountFor("/dev/shm") {
		shmPath, err := c.ShmResourcePath()
		if err != nil {
			return err
		}

		if err := idtools.MkdirAllAs(shmPath, 0700, rootUID, rootGID); err != nil {
			return err
		}

		shmSize := container.DefaultSHMSize
		if c.HostConfig.ShmSize != 0 {
			shmSize = c.HostConfig.ShmSize
		}
		shmproperty := "mode=1777,size=" + strconv.FormatInt(shmSize, 10)
		if err := syscall.Mount("shm", shmPath, "tmpfs", uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV), label.FormatMountLabel(shmproperty, c.GetMountLabel())); err != nil {
			return fmt.Errorf("mounting shm tmpfs: %s", err)
		}
		if err := os.Chown(shmPath, rootUID, rootGID); err != nil {
			return err
		}
	}

	return nil
}

func (daemon *Daemon) mountVolumes(container *container.Container) error {
	mounts, err := daemon.setupMounts(container)
	if err != nil {
		return err
	}

	for _, m := range mounts {
		dest, err := container.GetResourcePath(m.Destination)
		if err != nil {
			return err
		}

		var stat os.FileInfo
		stat, err = os.Stat(m.Source)
		if err != nil {
			return err
		}
		if err = fileutils.CreateIfNotExists(dest, stat.IsDir()); err != nil {
			return err
		}

		opts := "rbind,ro"
		if m.Writable {
			opts = "rbind,rw"
		}

		if err := mount.Mount(m.Source, dest, "bind", opts); err != nil {
			return err
		}
	}

	return nil
}

func killProcessDirectly(container *container.Container) error {
	if _, err := container.WaitStop(10 * time.Second); err != nil {
		// Ensure that we don't kill ourselves
		if pid := container.GetPID(); pid != 0 {
			logrus.Infof("Container %s failed to exit within 10 seconds of kill - trying direct SIGKILL", stringid.TruncateID(container.ID))
			if err := syscall.Kill(pid, 9); err != nil {
				if err != syscall.ESRCH {
					return err
				}
				logrus.Debugf("Cannot kill process (pid=%d) with signal 9: no such process.", pid)
			}
		}
	}
	return nil
}

func getDevicesFromPath(deviceMapping containertypes.DeviceMapping) (devs []*configs.Device, err error) {
	device, err := devices.DeviceFromPath(deviceMapping.PathOnHost, deviceMapping.CgroupPermissions)
	// if there was no error, return the device
	if err == nil {
		device.Path = deviceMapping.PathInContainer
		return append(devs, device), nil
	}

	// if the device is not a device node
	// try to see if it's a directory holding many devices
	if err == devices.ErrNotADevice {

		// check if it is a directory
		if src, e := os.Stat(deviceMapping.PathOnHost); e == nil && src.IsDir() {

			// mount the internal devices recursively
			filepath.Walk(deviceMapping.PathOnHost, func(dpath string, f os.FileInfo, e error) error {
				childDevice, e := devices.DeviceFromPath(dpath, deviceMapping.CgroupPermissions)
				if e != nil {
					// ignore the device
					return nil
				}

				// add the device to userSpecified devices
				childDevice.Path = strings.Replace(dpath, deviceMapping.PathOnHost, deviceMapping.PathInContainer, 1)
				devs = append(devs, childDevice)

				return nil
			})
		}
	}

	if len(devs) > 0 {
		return devs, nil
	}

	return devs, derr.ErrorCodeDeviceInfo.WithArgs(deviceMapping.PathOnHost, err)
}

func mergeDevices(defaultDevices, userDevices []*configs.Device) []*configs.Device {
	if len(userDevices) == 0 {
		return defaultDevices
	}

	paths := map[string]*configs.Device{}
	for _, d := range userDevices {
		paths[d.Path] = d
	}

	var devs []*configs.Device
	for _, d := range defaultDevices {
		if _, defined := paths[d.Path]; !defined {
			devs = append(devs, d)
		}
	}
	return append(devs, userDevices...)
}

func detachMounted(path string) error {
	return syscall.Unmount(path, syscall.MNT_DETACH)
}

func isLinkable(child *container.Container) bool {
	// A container is linkable only if it belongs to the default network
	_, ok := child.NetworkSettings.Networks["bridge"]
	return ok
}
