// +build linux freebsd

package daemon

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/links"
	"github.com/docker/docker/daemon/network"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/volume"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/label"
)

const (
	// DefaultPathEnv is unix style list of directories to search for
	// executables. Each directory is separated from the next by a colon
	// ':' character .
	DefaultPathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	// DefaultSHMSize is the default size (64MB) of the SHM which will be mounted in the container
	DefaultSHMSize int64 = 67108864
)

// Container holds the fields specific to unixen implementations. See
// CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	// Fields below here are platform specific.
	activeLinks     map[string]*links.Link
	AppArmorProfile string
	HostnamePath    string
	HostsPath       string
	ShmPath         string
	MqueuePath      string
	ResolvConfPath  string
}

func killProcessDirectly(container *Container) error {
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

func (daemon *Daemon) setupLinkedContainers(container *Container) ([]string, error) {
	var env []string
	children, err := daemon.children(container.Name)
	if err != nil {
		return nil, err
	}

	bridgeSettings := container.NetworkSettings.Networks["bridge"]
	if bridgeSettings == nil {
		return nil, nil
	}

	if len(children) > 0 {
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
	}
	return env, nil
}

func (container *Container) createDaemonEnvironment(linkedEnv []string) []string {
	// if a domain name was specified, append it to the hostname (see #7851)
	fullHostname := container.Config.Hostname
	if container.Config.Domainname != "" {
		fullHostname = fmt.Sprintf("%s.%s", fullHostname, container.Config.Domainname)
	}
	// Setup environment
	env := []string{
		"PATH=" + DefaultPathEnv,
		"HOSTNAME=" + fullHostname,
		// Note: we don't set HOME here because it'll get autoset intelligently
		// based on the value of USER inside dockerinit, but only if it isn't
		// set already (ie, that can be overridden by setting HOME via -e or ENV
		// in a Dockerfile).
	}
	if container.Config.Tty {
		env = append(env, "TERM=xterm")
	}
	env = append(env, linkedEnv...)
	// because the env on the container can override certain default values
	// we need to replace the 'env' keys where they match and append anything
	// else.
	env = utils.ReplaceOrAppendEnvValues(env, container.Config.Env)

	return env
}

func getDevicesFromPath(deviceMapping runconfig.DeviceMapping) (devs []*configs.Device, err error) {
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

func (daemon *Daemon) populateCommand(c *Container, env []string) error {
	var en *execdriver.Network
	if !c.Config.NetworkDisabled {
		en = &execdriver.Network{}
		if !daemon.execDriver.SupportsHooks() || c.hostConfig.NetworkMode.IsHost() {
			en.NamespacePath = c.NetworkSettings.SandboxKey
		}

		if c.hostConfig.NetworkMode.IsContainer() {
			nc, err := daemon.getNetworkedContainer(c.ID, c.hostConfig.NetworkMode.ConnectedContainer())
			if err != nil {
				return err
			}
			en.ContainerID = nc.ID
		}
	}

	ipc := &execdriver.Ipc{}
	var err error
	c.ShmPath, err = c.shmPath()
	if err != nil {
		return err
	}

	c.MqueuePath, err = c.mqueuePath()
	if err != nil {
		return err
	}

	if c.hostConfig.IpcMode.IsContainer() {
		ic, err := daemon.getIpcContainer(c)
		if err != nil {
			return err
		}
		ipc.ContainerID = ic.ID
		c.ShmPath = ic.ShmPath
		c.MqueuePath = ic.MqueuePath
	} else {
		ipc.HostIpc = c.hostConfig.IpcMode.IsHost()
		if ipc.HostIpc {
			if _, err := os.Stat("/dev/shm"); err != nil {
				return fmt.Errorf("/dev/shm is not mounted, but must be for --ipc=host")
			}
			if _, err := os.Stat("/dev/mqueue"); err != nil {
				return fmt.Errorf("/dev/mqueue is not mounted, but must be for --ipc=host")
			}
			c.ShmPath = "/dev/shm"
			c.MqueuePath = "/dev/mqueue"
		}
	}

	pid := &execdriver.Pid{}
	pid.HostPid = c.hostConfig.PidMode.IsHost()

	uts := &execdriver.UTS{
		HostUTS: c.hostConfig.UTSMode.IsHost(),
	}

	// Build lists of devices allowed and created within the container.
	var userSpecifiedDevices []*configs.Device
	for _, deviceMapping := range c.hostConfig.Devices {
		devs, err := getDevicesFromPath(deviceMapping)
		if err != nil {
			return err
		}

		userSpecifiedDevices = append(userSpecifiedDevices, devs...)
	}

	allowedDevices := mergeDevices(configs.DefaultAllowedDevices, userSpecifiedDevices)

	autoCreatedDevices := mergeDevices(configs.DefaultAutoCreatedDevices, userSpecifiedDevices)

	var rlimits []*ulimit.Rlimit
	ulimits := c.hostConfig.Ulimits

	// Merge ulimits with daemon defaults
	ulIdx := make(map[string]*ulimit.Ulimit)
	for _, ul := range ulimits {
		ulIdx[ul.Name] = ul
	}
	for name, ul := range daemon.configStore.Ulimits {
		if _, exists := ulIdx[name]; !exists {
			ulimits = append(ulimits, ul)
		}
	}

	weightDevices, err := getBlkioWeightDevices(c.hostConfig)
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
			Memory:            c.hostConfig.Memory,
			MemoryReservation: c.hostConfig.MemoryReservation,
			CPUShares:         c.hostConfig.CPUShares,
			BlkioWeight:       c.hostConfig.BlkioWeight,
		},
		MemorySwap:        c.hostConfig.MemorySwap,
		KernelMemory:      c.hostConfig.KernelMemory,
		CpusetCpus:        c.hostConfig.CpusetCpus,
		CpusetMems:        c.hostConfig.CpusetMems,
		CPUPeriod:         c.hostConfig.CPUPeriod,
		CPUQuota:          c.hostConfig.CPUQuota,
		Rlimits:           rlimits,
		BlkioWeightDevice: weightDevices,
		OomKillDisable:    c.hostConfig.OomKillDisable,
		MemorySwappiness:  -1,
	}

	if c.hostConfig.MemorySwappiness != nil {
		resources.MemorySwappiness = *c.hostConfig.MemorySwappiness
	}

	processConfig := execdriver.ProcessConfig{
		CommonProcessConfig: execdriver.CommonProcessConfig{
			Entrypoint: c.Path,
			Arguments:  c.Args,
			Tty:        c.Config.Tty,
		},
		Privileged: c.hostConfig.Privileged,
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

	c.command = &execdriver.Command{
		CommonCommand: execdriver.CommonCommand{
			ID:            c.ID,
			InitPath:      "/.dockerinit",
			MountLabel:    c.getMountLabel(),
			Network:       en,
			ProcessConfig: processConfig,
			ProcessLabel:  c.getProcessLabel(),
			Rootfs:        c.rootfsPath(),
			Resources:     resources,
			WorkingDir:    c.Config.WorkingDir,
		},
		AllowedDevices:     allowedDevices,
		AppArmorProfile:    c.AppArmorProfile,
		AutoCreatedDevices: autoCreatedDevices,
		CapAdd:             c.hostConfig.CapAdd.Slice(),
		CapDrop:            c.hostConfig.CapDrop.Slice(),
		CgroupParent:       c.hostConfig.CgroupParent,
		GIDMapping:         gidMap,
		GroupAdd:           c.hostConfig.GroupAdd,
		Ipc:                ipc,
		Pid:                pid,
		ReadonlyRootfs:     c.hostConfig.ReadonlyRootfs,
		RemappedRoot:       remappedRoot,
		UIDMapping:         uidMap,
		UTS:                uts,
	}

	return nil
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

// getSize returns the real size & virtual size of the container.
func (daemon *Daemon) getSize(container *Container) (int64, int64) {
	var (
		sizeRw, sizeRootfs int64
		err                error
	)

	if err := daemon.Mount(container); err != nil {
		logrus.Errorf("Failed to compute size of container rootfs %s: %s", container.ID, err)
		return sizeRw, sizeRootfs
	}
	defer daemon.Unmount(container)

	sizeRw, err = container.rwlayer.Size()
	if err != nil {
		logrus.Errorf("Driver %s couldn't return diff size of container %s: %s", daemon.driver, container.ID, err)
		// FIXME: GetSize should return an error. Not changing it now in case
		// there is a side-effect.
		sizeRw = -1
	}

	if parent := container.rwlayer.Parent(); parent != nil {
		sizeRootfs, err = parent.Size()
		if err != nil {
			sizeRootfs = -1
		} else if sizeRw != -1 {
			sizeRootfs += sizeRw
		}
	}
	return sizeRw, sizeRootfs
}

// Attempt to set the network mounts given a provided destination and
// the path to use for it; return true if the given destination was a
// network mount file
func (container *Container) trySetNetworkMount(destination string, path string) bool {
	if destination == "/etc/resolv.conf" {
		container.ResolvConfPath = path
		return true
	}
	if destination == "/etc/hostname" {
		container.HostnamePath = path
		return true
	}
	if destination == "/etc/hosts" {
		container.HostsPath = path
		return true
	}

	return false
}

func (container *Container) buildHostnameFile() error {
	hostnamePath, err := container.getRootResourcePath("hostname")
	if err != nil {
		return err
	}
	container.HostnamePath = hostnamePath

	if container.Config.Domainname != "" {
		return ioutil.WriteFile(container.HostnamePath, []byte(fmt.Sprintf("%s.%s\n", container.Config.Hostname, container.Config.Domainname)), 0644)
	}
	return ioutil.WriteFile(container.HostnamePath, []byte(container.Config.Hostname+"\n"), 0644)
}

func (daemon *Daemon) buildSandboxOptions(container *Container, n libnetwork.Network) ([]libnetwork.SandboxOption, error) {
	var (
		sboxOptions []libnetwork.SandboxOption
		err         error
		dns         []string
		dnsSearch   []string
		dnsOptions  []string
	)

	sboxOptions = append(sboxOptions, libnetwork.OptionHostname(container.Config.Hostname),
		libnetwork.OptionDomainname(container.Config.Domainname))

	if container.hostConfig.NetworkMode.IsHost() {
		sboxOptions = append(sboxOptions, libnetwork.OptionUseDefaultSandbox())
		sboxOptions = append(sboxOptions, libnetwork.OptionOriginHostsPath("/etc/hosts"))
		sboxOptions = append(sboxOptions, libnetwork.OptionOriginResolvConfPath("/etc/resolv.conf"))
	} else if daemon.execDriver.SupportsHooks() {
		// OptionUseExternalKey is mandatory for userns support.
		// But optional for non-userns support
		sboxOptions = append(sboxOptions, libnetwork.OptionUseExternalKey())
	}

	container.HostsPath, err = container.getRootResourcePath("hosts")
	if err != nil {
		return nil, err
	}
	sboxOptions = append(sboxOptions, libnetwork.OptionHostsPath(container.HostsPath))

	container.ResolvConfPath, err = container.getRootResourcePath("resolv.conf")
	if err != nil {
		return nil, err
	}
	sboxOptions = append(sboxOptions, libnetwork.OptionResolvConfPath(container.ResolvConfPath))

	if len(container.hostConfig.DNS) > 0 {
		dns = container.hostConfig.DNS
	} else if len(daemon.configStore.DNS) > 0 {
		dns = daemon.configStore.DNS
	}

	for _, d := range dns {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNS(d))
	}

	if len(container.hostConfig.DNSSearch) > 0 {
		dnsSearch = container.hostConfig.DNSSearch
	} else if len(daemon.configStore.DNSSearch) > 0 {
		dnsSearch = daemon.configStore.DNSSearch
	}

	for _, ds := range dnsSearch {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSSearch(ds))
	}

	if len(container.hostConfig.DNSOptions) > 0 {
		dnsOptions = container.hostConfig.DNSOptions
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

	for _, extraHost := range container.hostConfig.ExtraHosts {
		// allow IPv6 addresses in extra hosts; only split on first ":"
		parts := strings.SplitN(extraHost, ":", 2)
		sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(parts[0], parts[1]))
	}

	// Link feature is supported only for the default bridge network.
	// return if this call to build join options is not for default bridge network
	if n.Name() != "bridge" {
		return sboxOptions, nil
	}

	ep, _ := container.getEndpointInNetwork(n)
	if ep == nil {
		return sboxOptions, nil
	}

	var childEndpoints, parentEndpoints []string

	children, err := daemon.children(container.Name)
	if err != nil {
		return nil, err
	}

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
		cEndpoint, _ := child.getEndpointInNetwork(n)
		if cEndpoint != nil && cEndpoint.ID() != "" {
			childEndpoints = append(childEndpoints, cEndpoint.ID())
		}
	}

	bridgeSettings := container.NetworkSettings.Networks["bridge"]
	refs := daemon.containerGraph().RefPaths(container.ID)
	for _, ref := range refs {
		if ref.ParentID == "0" {
			continue
		}

		c, err := daemon.Get(ref.ParentID)
		if err != nil {
			logrus.Error(err)
		}

		if c != nil && !daemon.configStore.DisableBridge && container.hostConfig.NetworkMode.IsPrivate() {
			logrus.Debugf("Update /etc/hosts of %s for alias %s with ip %s", c.ID, ref.Name, bridgeSettings.IPAddress)
			sboxOptions = append(sboxOptions, libnetwork.OptionParentUpdate(c.ID, ref.Name, bridgeSettings.IPAddress))
			if ep.ID() != "" {
				parentEndpoints = append(parentEndpoints, ep.ID())
			}
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

func isLinkable(child *Container) bool {
	// A container is linkable only if it belongs to the default network
	_, ok := child.NetworkSettings.Networks["bridge"]
	return ok
}

func (container *Container) getEndpointInNetwork(n libnetwork.Network) (libnetwork.Endpoint, error) {
	endpointName := strings.TrimPrefix(container.Name, "/")
	return n.EndpointByName(endpointName)
}

func (container *Container) buildPortMapInfo(ep libnetwork.Endpoint, networkSettings *network.Settings) (*network.Settings, error) {
	if ep == nil {
		return nil, derr.ErrorCodeEmptyEndpoint
	}

	if networkSettings == nil {
		return nil, derr.ErrorCodeEmptyNetwork
	}

	driverInfo, err := ep.DriverInfo()
	if err != nil {
		return nil, err
	}

	if driverInfo == nil {
		// It is not an error for epInfo to be nil
		return networkSettings, nil
	}

	if networkSettings.Ports == nil {
		networkSettings.Ports = nat.PortMap{}
	}

	if expData, ok := driverInfo[netlabel.ExposedPorts]; ok {
		if exposedPorts, ok := expData.([]types.TransportPort); ok {
			for _, tp := range exposedPorts {
				natPort, err := nat.NewPort(tp.Proto.String(), strconv.Itoa(int(tp.Port)))
				if err != nil {
					return nil, derr.ErrorCodeParsingPort.WithArgs(tp.Port, err)
				}
				networkSettings.Ports[natPort] = nil
			}
		}
	}

	mapData, ok := driverInfo[netlabel.PortMap]
	if !ok {
		return networkSettings, nil
	}

	if portMapping, ok := mapData.([]types.PortBinding); ok {
		for _, pp := range portMapping {
			natPort, err := nat.NewPort(pp.Proto.String(), strconv.Itoa(int(pp.Port)))
			if err != nil {
				return nil, err
			}
			natBndg := nat.PortBinding{HostIP: pp.HostIP.String(), HostPort: strconv.Itoa(int(pp.HostPort))}
			networkSettings.Ports[natPort] = append(networkSettings.Ports[natPort], natBndg)
		}
	}

	return networkSettings, nil
}

func (container *Container) buildEndpointInfo(n libnetwork.Network, ep libnetwork.Endpoint, networkSettings *network.Settings) (*network.Settings, error) {
	if ep == nil {
		return nil, derr.ErrorCodeEmptyEndpoint
	}

	if networkSettings == nil {
		return nil, derr.ErrorCodeEmptyNetwork
	}

	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return networkSettings, nil
	}

	if _, ok := networkSettings.Networks[n.Name()]; !ok {
		networkSettings.Networks[n.Name()] = new(network.EndpointSettings)
	}
	networkSettings.Networks[n.Name()].EndpointID = ep.ID()

	iface := epInfo.Iface()
	if iface == nil {
		return networkSettings, nil
	}

	if iface.MacAddress() != nil {
		networkSettings.Networks[n.Name()].MacAddress = iface.MacAddress().String()
	}

	if iface.Address() != nil {
		ones, _ := iface.Address().Mask.Size()
		networkSettings.Networks[n.Name()].IPAddress = iface.Address().IP.String()
		networkSettings.Networks[n.Name()].IPPrefixLen = ones
	}

	if iface.AddressIPv6() != nil && iface.AddressIPv6().IP.To16() != nil {
		onesv6, _ := iface.AddressIPv6().Mask.Size()
		networkSettings.Networks[n.Name()].GlobalIPv6Address = iface.AddressIPv6().IP.String()
		networkSettings.Networks[n.Name()].GlobalIPv6PrefixLen = onesv6
	}

	return networkSettings, nil
}

func (container *Container) updateJoinInfo(n libnetwork.Network, ep libnetwork.Endpoint) error {
	if _, err := container.buildPortMapInfo(ep, container.NetworkSettings); err != nil {
		return err
	}

	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return nil
	}
	if epInfo.Gateway() != nil {
		container.NetworkSettings.Networks[n.Name()].Gateway = epInfo.Gateway().String()
	}
	if epInfo.GatewayIPv6().To16() != nil {
		container.NetworkSettings.Networks[n.Name()].IPv6Gateway = epInfo.GatewayIPv6().String()
	}

	return nil
}

func (daemon *Daemon) updateNetworkSettings(container *Container, n libnetwork.Network) error {
	if container.NetworkSettings == nil {
		container.NetworkSettings = &network.Settings{Networks: make(map[string]*network.EndpointSettings)}
	}

	if !container.hostConfig.NetworkMode.IsHost() && runconfig.NetworkMode(n.Type()).IsHost() {
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
		if !runconfig.NetworkMode(sn.Type()).IsPrivate() ||
			!runconfig.NetworkMode(n.Type()).IsPrivate() {
			return runconfig.ErrConflictSharedNetwork
		}
		if runconfig.NetworkMode(sn.Name()).IsNone() ||
			runconfig.NetworkMode(n.Name()).IsNone() {
			return runconfig.ErrConflictNoNetwork
		}
	}
	container.NetworkSettings.Networks[n.Name()] = new(network.EndpointSettings)

	return nil
}

func (daemon *Daemon) updateEndpointNetworkSettings(container *Container, n libnetwork.Network, ep libnetwork.Endpoint) error {
	networkSettings, err := container.buildEndpointInfo(n, ep, container.NetworkSettings)
	if err != nil {
		return err
	}

	if container.hostConfig.NetworkMode == runconfig.NetworkMode("bridge") {
		networkSettings.Bridge = daemon.configStore.Bridge.Iface
	}

	return nil
}

func (container *Container) updateSandboxNetworkSettings(sb libnetwork.Sandbox) error {
	container.NetworkSettings.SandboxID = sb.ID()
	container.NetworkSettings.SandboxKey = sb.Key()
	return nil
}

// UpdateNetwork is used to update the container's network (e.g. when linked containers
// get removed/unlinked).
func (daemon *Daemon) updateNetwork(container *Container) error {
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

func (container *Container) buildCreateEndpointOptions(n libnetwork.Network) ([]libnetwork.EndpointOption, error) {
	var (
		portSpecs     = make(nat.PortSet)
		bindings      = make(nat.PortMap)
		pbList        []types.PortBinding
		exposeList    []types.TransportPort
		createOptions []libnetwork.EndpointOption
	)

	if n.Name() == "bridge" || container.NetworkSettings.IsAnonymousEndpoint {
		createOptions = append(createOptions, libnetwork.CreateOptionAnonymous())
	}

	// Other configs are applicable only for the endpoint in the network
	// to which container was connected to on docker run.
	if n.Name() != container.hostConfig.NetworkMode.NetworkName() &&
		!(n.Name() == "bridge" && container.hostConfig.NetworkMode.IsDefault()) {
		return createOptions, nil
	}

	if container.Config.ExposedPorts != nil {
		portSpecs = container.Config.ExposedPorts
	}

	if container.hostConfig.PortBindings != nil {
		for p, b := range container.hostConfig.PortBindings {
			bindings[p] = []nat.PortBinding{}
			for _, bb := range b {
				bindings[p] = append(bindings[p], nat.PortBinding{
					HostIP:   bb.HostIP,
					HostPort: bb.HostPort,
				})
			}
		}
	}

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
				return nil, derr.ErrorCodeHostPort.WithArgs(binding[i].HostPort, err)
			}
			pbCopy.HostPort = uint16(portStart)
			pbCopy.HostPortEnd = uint16(portEnd)
			pbCopy.HostIP = net.ParseIP(binding[i].HostIP)
			pbList = append(pbList, pbCopy)
		}

		if container.hostConfig.PublishAllPorts && len(binding) == 0 {
			pbList = append(pbList, pb)
		}
	}

	createOptions = append(createOptions,
		libnetwork.CreateOptionPortMapping(pbList),
		libnetwork.CreateOptionExposedPorts(exposeList))

	if container.Config.MacAddress != "" {
		mac, err := net.ParseMAC(container.Config.MacAddress)
		if err != nil {
			return nil, err
		}

		genericOption := options.Generic{
			netlabel.MacAddress: mac,
		}

		createOptions = append(createOptions, libnetwork.EndpointOptionGeneric(genericOption))
	}

	return createOptions, nil
}

func (daemon *Daemon) allocateNetwork(container *Container) error {
	controller := daemon.netController

	// Cleanup any stale sandbox left over due to ungraceful daemon shutdown
	if err := controller.SandboxDestroy(container.ID); err != nil {
		logrus.Errorf("failed to cleanup up stale network sandbox for container %s", container.ID)
	}

	updateSettings := false
	if len(container.NetworkSettings.Networks) == 0 {
		mode := container.hostConfig.NetworkMode
		if container.Config.NetworkDisabled || mode.IsContainer() {
			return nil
		}

		networkName := mode.NetworkName()
		if mode.IsDefault() {
			networkName = controller.Config().Daemon.DefaultNetwork
		}
		if mode.IsUserDefined() {
			n, err := daemon.FindNetwork(networkName)
			if err != nil {
				return err
			}
			networkName = n.Name()
		}
		container.NetworkSettings.Networks = make(map[string]*network.EndpointSettings)
		container.NetworkSettings.Networks[networkName] = new(network.EndpointSettings)
		updateSettings = true
	}

	for n := range container.NetworkSettings.Networks {
		if err := daemon.connectToNetwork(container, n, updateSettings); err != nil {
			return err
		}
	}

	return container.writeHostConfig()
}

func (daemon *Daemon) getNetworkSandbox(container *Container) libnetwork.Sandbox {
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

// ConnectToNetwork connects a container to a network
func (daemon *Daemon) ConnectToNetwork(container *Container, idOrName string) error {
	if !container.Running {
		return derr.ErrorCodeNotRunning.WithArgs(container.ID)
	}
	if err := daemon.connectToNetwork(container, idOrName, true); err != nil {
		return err
	}
	if err := container.toDiskLocking(); err != nil {
		return fmt.Errorf("Error saving container to disk: %v", err)
	}
	return nil
}

func (daemon *Daemon) connectToNetwork(container *Container, idOrName string, updateSettings bool) (err error) {
	if container.hostConfig.NetworkMode.IsContainer() {
		return runconfig.ErrConflictSharedNetwork
	}

	if runconfig.NetworkMode(idOrName).IsBridge() &&
		daemon.configStore.DisableBridge {
		container.Config.NetworkDisabled = true
		return nil
	}

	controller := daemon.netController

	n, err := daemon.FindNetwork(idOrName)
	if err != nil {
		return err
	}

	if updateSettings {
		if err := daemon.updateNetworkSettings(container, n); err != nil {
			return err
		}
	}

	ep, err := container.getEndpointInNetwork(n)
	if err == nil {
		return fmt.Errorf("container already connected to network %s", idOrName)
	}

	if _, ok := err.(libnetwork.ErrNoSuchEndpoint); !ok {
		return err
	}

	createOptions, err := container.buildCreateEndpointOptions(n)
	if err != nil {
		return err
	}

	endpointName := strings.TrimPrefix(container.Name, "/")
	ep, err = n.CreateEndpoint(endpointName, createOptions...)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if e := ep.Delete(); e != nil {
				logrus.Warnf("Could not rollback container connection to network %s", idOrName)
			}
		}
	}()

	if err := daemon.updateEndpointNetworkSettings(container, n, ep); err != nil {
		return err
	}

	sb := daemon.getNetworkSandbox(container)
	if sb == nil {
		options, err := daemon.buildSandboxOptions(container, n)
		if err != nil {
			return err
		}
		sb, err = controller.NewSandbox(container.ID, options...)
		if err != nil {
			return err
		}

		container.updateSandboxNetworkSettings(sb)
	}

	if err := ep.Join(sb); err != nil {
		return err
	}

	if err := container.updateJoinInfo(n, ep); err != nil {
		return derr.ErrorCodeJoinInfo.WithArgs(err)
	}

	return nil
}

func (daemon *Daemon) initializeNetworking(container *Container) error {
	var err error

	if container.hostConfig.NetworkMode.IsContainer() {
		// we need to get the hosts files from the container to join
		nc, err := daemon.getNetworkedContainer(container.ID, container.hostConfig.NetworkMode.ConnectedContainer())
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

	if container.hostConfig.NetworkMode.IsHost() {
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

	return container.buildHostnameFile()
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

func (daemon *Daemon) getIpcContainer(container *Container) (*Container, error) {
	containerID := container.hostConfig.IpcMode.Container()
	c, err := daemon.Get(containerID)
	if err != nil {
		return nil, err
	}
	if !c.IsRunning() {
		return nil, derr.ErrorCodeIPCRunning
	}
	return c, nil
}

func (container *Container) setupWorkingDirectory() error {
	if container.Config.WorkingDir == "" {
		return nil
	}
	container.Config.WorkingDir = filepath.Clean(container.Config.WorkingDir)

	pth, err := container.GetResourcePath(container.Config.WorkingDir)
	if err != nil {
		return err
	}

	pthInfo, err := os.Stat(pth)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		if err := system.MkdirAll(pth, 0755); err != nil {
			return err
		}
	}
	if pthInfo != nil && !pthInfo.IsDir() {
		return derr.ErrorCodeNotADir.WithArgs(container.Config.WorkingDir)
	}
	return nil
}

func (daemon *Daemon) getNetworkedContainer(containerID, connectedContainerID string) (*Container, error) {
	nc, err := daemon.Get(connectedContainerID)
	if err != nil {
		return nil, err
	}
	if containerID == nc.ID {
		return nil, derr.ErrorCodeJoinSelf
	}
	if !nc.IsRunning() {
		return nil, derr.ErrorCodeJoinRunning.WithArgs(connectedContainerID)
	}
	return nc, nil
}

func (daemon *Daemon) releaseNetwork(container *Container) {
	if container.hostConfig.NetworkMode.IsContainer() || container.Config.NetworkDisabled {
		return
	}

	sid := container.NetworkSettings.SandboxID
	networks := container.NetworkSettings.Networks
	for n := range networks {
		networks[n] = &network.EndpointSettings{}
	}

	container.NetworkSettings = &network.Settings{Networks: networks}

	if sid == "" || len(networks) == 0 {
		return
	}

	sb, err := daemon.netController.SandboxByID(sid)
	if err != nil {
		logrus.Errorf("error locating sandbox id %s: %v", sid, err)
		return
	}

	if err := sb.Delete(); err != nil {
		logrus.Errorf("Error deleting sandbox id %s for container %s: %v", sid, container.ID, err)
	}
}

// DisconnectFromNetwork disconnects a container from a network
func (container *Container) DisconnectFromNetwork(n libnetwork.Network) error {
	if !container.Running {
		return derr.ErrorCodeNotRunning.WithArgs(container.ID)
	}

	if container.hostConfig.NetworkMode.IsHost() && runconfig.NetworkMode(n.Type()).IsHost() {
		return runconfig.ErrConflictHostNetwork
	}

	if err := container.disconnectFromNetwork(n); err != nil {
		return err
	}

	if err := container.toDiskLocking(); err != nil {
		return fmt.Errorf("Error saving container to disk: %v", err)
	}
	return nil
}

func (container *Container) disconnectFromNetwork(n libnetwork.Network) error {
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

	if ep == nil {
		return fmt.Errorf("container %s is not connected to the network", container.ID)
	}

	if err := ep.Leave(sbox); err != nil {
		return fmt.Errorf("container %s failed to leave network %s: %v", container.ID, n.Name(), err)
	}

	if err := ep.Delete(); err != nil {
		return fmt.Errorf("endpoint delete failed for container %s on network %s: %v", container.ID, n.Name(), err)
	}

	delete(container.NetworkSettings.Networks, n.Name())
	return nil
}

// appendNetworkMounts appends any network mounts to the array of mount points passed in
func appendNetworkMounts(container *Container, volumeMounts []volume.MountPoint) ([]volume.MountPoint, error) {
	for _, mnt := range container.networkMounts() {
		dest, err := container.GetResourcePath(mnt.Destination)
		if err != nil {
			return nil, err
		}
		volumeMounts = append(volumeMounts, volume.MountPoint{Destination: dest})
	}
	return volumeMounts, nil
}

func (container *Container) networkMounts() []execdriver.Mount {
	var mounts []execdriver.Mount
	shared := container.hostConfig.NetworkMode.IsContainer()
	if container.ResolvConfPath != "" {
		if _, err := os.Stat(container.ResolvConfPath); err != nil {
			logrus.Warnf("ResolvConfPath set to %q, but can't stat this filename (err = %v); skipping", container.ResolvConfPath, err)
		} else {
			label.Relabel(container.ResolvConfPath, container.MountLabel, shared)
			writable := !container.hostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/resolv.conf"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.ResolvConfPath,
				Destination: "/etc/resolv.conf",
				Writable:    writable,
				Private:     true,
			})
		}
	}
	if container.HostnamePath != "" {
		if _, err := os.Stat(container.HostnamePath); err != nil {
			logrus.Warnf("HostnamePath set to %q, but can't stat this filename (err = %v); skipping", container.HostnamePath, err)
		} else {
			label.Relabel(container.HostnamePath, container.MountLabel, shared)
			writable := !container.hostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/hostname"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.HostnamePath,
				Destination: "/etc/hostname",
				Writable:    writable,
				Private:     true,
			})
		}
	}
	if container.HostsPath != "" {
		if _, err := os.Stat(container.HostsPath); err != nil {
			logrus.Warnf("HostsPath set to %q, but can't stat this filename (err = %v); skipping", container.HostsPath, err)
		} else {
			label.Relabel(container.HostsPath, container.MountLabel, shared)
			writable := !container.hostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/hosts"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.HostsPath,
				Destination: "/etc/hosts",
				Writable:    writable,
				Private:     true,
			})
		}
	}
	return mounts
}

func (container *Container) copyImagePathContent(v volume.Volume, destination string) error {
	rootfs, err := symlink.FollowSymlinkInScope(filepath.Join(container.basefs, destination), container.basefs)
	if err != nil {
		return err
	}

	if _, err = ioutil.ReadDir(rootfs); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	path, err := v.Mount()
	if err != nil {
		return err
	}

	if err := copyExistingContents(rootfs, path); err != nil {
		return err
	}

	return v.Unmount()
}

func (container *Container) shmPath() (string, error) {
	return container.getRootResourcePath("shm")
}
func (container *Container) mqueuePath() (string, error) {
	return container.getRootResourcePath("mqueue")
}

func (container *Container) hasMountFor(path string) bool {
	_, exists := container.MountPoints[path]
	return exists
}

func (daemon *Daemon) setupIpcDirs(container *Container) error {
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	if !container.hasMountFor("/dev/shm") {
		shmPath, err := container.shmPath()
		if err != nil {
			return err
		}

		if err := idtools.MkdirAllAs(shmPath, 0700, rootUID, rootGID); err != nil {
			return err
		}

		shmSize := DefaultSHMSize
		if container.hostConfig.ShmSize != nil {
			shmSize = *container.hostConfig.ShmSize
		}

		shmproperty := "mode=1777,size=" + strconv.FormatInt(shmSize, 10)
		if err := syscall.Mount("shm", shmPath, "tmpfs", uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV), label.FormatMountLabel(shmproperty, container.getMountLabel())); err != nil {
			return fmt.Errorf("mounting shm tmpfs: %s", err)
		}
		if err := os.Chown(shmPath, rootUID, rootGID); err != nil {
			return err
		}
	}

	if !container.hasMountFor("/dev/mqueue") {
		mqueuePath, err := container.mqueuePath()
		if err != nil {
			return err
		}

		if err := idtools.MkdirAllAs(mqueuePath, 0700, rootUID, rootGID); err != nil {
			return err
		}

		if err := syscall.Mount("mqueue", mqueuePath, "mqueue", uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV), ""); err != nil {
			return fmt.Errorf("mounting mqueue mqueue : %s", err)
		}
		if err := os.Chown(mqueuePath, rootUID, rootGID); err != nil {
			return err
		}
	}

	return nil
}

func (container *Container) unmountIpcMounts(unmount func(pth string) error) {
	if container.hostConfig.IpcMode.IsContainer() || container.hostConfig.IpcMode.IsHost() {
		return
	}

	var warnings []string

	if !container.hasMountFor("/dev/shm") {
		shmPath, err := container.shmPath()
		if err != nil {
			logrus.Error(err)
			warnings = append(warnings, err.Error())
		} else if shmPath != "" {
			if err := unmount(shmPath); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to umount %s: %v", shmPath, err))
			}

		}
	}

	if !container.hasMountFor("/dev/mqueue") {
		mqueuePath, err := container.mqueuePath()
		if err != nil {
			logrus.Error(err)
			warnings = append(warnings, err.Error())
		} else if mqueuePath != "" {
			if err := unmount(mqueuePath); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to umount %s: %v", mqueuePath, err))
			}
		}
	}

	if len(warnings) > 0 {
		logrus.Warnf("failed to cleanup ipc mounts:\n%v", strings.Join(warnings, "\n"))
	}
}

func (container *Container) ipcMounts() []execdriver.Mount {
	var mounts []execdriver.Mount

	if !container.hasMountFor("/dev/shm") {
		label.SetFileLabel(container.ShmPath, container.MountLabel)
		mounts = append(mounts, execdriver.Mount{
			Source:      container.ShmPath,
			Destination: "/dev/shm",
			Writable:    true,
			Private:     true,
		})
	}

	if !container.hasMountFor("/dev/mqueue") {
		label.SetFileLabel(container.MqueuePath, container.MountLabel)
		mounts = append(mounts, execdriver.Mount{
			Source:      container.MqueuePath,
			Destination: "/dev/mqueue",
			Writable:    true,
			Private:     true,
		})
	}
	return mounts
}

func detachMounted(path string) error {
	return syscall.Unmount(path, syscall.MNT_DETACH)
}

func (daemon *Daemon) mountVolumes(container *Container) error {
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

func (container *Container) unmountVolumes(forceSyscall bool) error {
	var (
		volumeMounts []volume.MountPoint
		err          error
	)

	for _, mntPoint := range container.MountPoints {
		dest, err := container.GetResourcePath(mntPoint.Destination)
		if err != nil {
			return err
		}

		volumeMounts = append(volumeMounts, volume.MountPoint{Destination: dest, Volume: mntPoint.Volume})
	}

	// Append any network mounts to the list (this is a no-op on Windows)
	if volumeMounts, err = appendNetworkMounts(container, volumeMounts); err != nil {
		return err
	}

	for _, volumeMount := range volumeMounts {
		if forceSyscall {
			if err := detachMounted(volumeMount.Destination); err != nil {
				logrus.Warnf("%s unmountVolumes: Failed to do lazy umount %v", container.ID, err)
			}
		}

		if volumeMount.Volume != nil {
			if err := volumeMount.Volume.Unmount(); err != nil {
				return err
			}
		}
	}

	return nil
}
