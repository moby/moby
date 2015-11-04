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
	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/links"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/stringid"
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

// DefaultPathEnv is unix style list of directories to search for
// executables. Each directory is separated from the next by a colon
// ':' character .
const DefaultPathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// Container holds the fields specific to unixen implementations. See
// CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	// Fields below here are platform specific.
	activeLinks     map[string]*links.Link
	AppArmorProfile string
	HostnamePath    string
	HostsPath       string
	MountPoints     map[string]*mountPoint
	ResolvConfPath  string

	Volumes   map[string]string // Deprecated since 1.7, kept for backwards compatibility
	VolumesRW map[string]bool   // Deprecated since 1.7, kept for backwards compatibility
}

func killProcessDirectly(container *Container) error {
	if _, err := container.WaitStop(10 * time.Second); err != nil {
		// Ensure that we don't kill ourselves
		if pid := container.getPID(); pid != 0 {
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

func (container *Container) setupLinkedContainers() ([]string, error) {
	var (
		env    []string
		daemon = container.daemon
	)
	children, err := daemon.children(container.Name)
	if err != nil {
		return nil, err
	}

	if len(children) > 0 {
		for linkAlias, child := range children {
			if !child.IsRunning() {
				return nil, derr.ErrorCodeLinkNotRunning.WithArgs(child.Name, linkAlias)
			}

			link := links.NewLink(
				container.NetworkSettings.IPAddress,
				child.NetworkSettings.IPAddress,
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

func Major(devNumber uint64) uint64 {
	return uint64((devNumber >> 8) & 0xfff)
}

func Minor(devNumber uint64) uint64 {
	return uint64((devNumber & 0xff) | ((devNumber >> 12) & 0xfff00))
}

func constructBlkioArgs(volumeMap []string, blkioLimit string) string {
	if blkioLimit == "" {
		return ""
	}
	if len(volumeMap) == 0 {
		volumeMap = append(volumeMap, "/var/lib/docker")
	}
	var dupRemovalMap = make(map[string]bool)
	for _, volumeMapping := range volumeMap {
		splitArr := strings.Split(volumeMapping, ":")
		f, _ := os.Open(splitArr[0])
		fi, _ := f.Stat()
		s := fi.Sys()
		switch s := s.(type) {
		default:
			fmt.Printf("unexpected type %T", s)
		case *syscall.Stat_t:
			majorMinorStr := strconv.FormatUint(Major(s.Dev), 10) + ":" + strconv.FormatUint(Minor(s.Dev), 10)
			dupRemovalMap[majorMinorStr] = true
		}
	}

	blkioArg := ""
	for key, _ := range dupRemovalMap {
		blkioArg = blkioArg + key + " " + blkioLimit + "\n"
	}
	return blkioArg
}

func populateCommand(c *Container, env []string) error {
	var en *execdriver.Network
	if !c.Config.NetworkDisabled {
		en = &execdriver.Network{}
		if !c.daemon.execDriver.SupportsHooks() || c.hostConfig.NetworkMode.IsHost() {
			en.NamespacePath = c.NetworkSettings.SandboxKey
		}

		parts := strings.SplitN(string(c.hostConfig.NetworkMode), ":", 2)
		if parts[0] == "container" {
			nc, err := c.getNetworkedContainer()
			if err != nil {
				return err
			}
			en.ContainerID = nc.ID
		}
	}

	ipc := &execdriver.Ipc{}

	if c.hostConfig.IpcMode.IsContainer() {
		ic, err := c.getIpcContainer()
		if err != nil {
			return err
		}
		ipc.ContainerID = ic.ID
	} else {
		ipc.HostIpc = c.hostConfig.IpcMode.IsHost()
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

	// TODO: this can be removed after lxc-conf is fully deprecated
	lxcConfig, err := mergeLxcConfIntoOptions(c.hostConfig)
	if err != nil {
		return err
	}

	var rlimits []*ulimit.Rlimit
	ulimits := c.hostConfig.Ulimits

	// Merge ulimits with daemon defaults
	ulIdx := make(map[string]*ulimit.Ulimit)
	for _, ul := range ulimits {
		ulIdx[ul.Name] = ul
	}
	for name, ul := range c.daemon.configStore.Ulimits {
		if _, exists := ulIdx[name]; !exists {
			ulimits = append(ulimits, ul)
		}
	}

	for _, limit := range ulimits {
		rl, err := limit.GetRlimit()
		if err != nil {
			return err
		}
		rlimits = append(rlimits, rl)
	}

	resources := &execdriver.Resources{
		Memory:           c.hostConfig.Memory,
		MemorySwap:       c.hostConfig.MemorySwap,
		KernelMemory:     c.hostConfig.KernelMemory,
		CPUShares:        c.hostConfig.CPUShares,
		CpusetCpus:       c.hostConfig.CpusetCpus,
		CpusetMems:       c.hostConfig.CpusetMems,
		CPUPeriod:        c.hostConfig.CPUPeriod,
		CPUQuota:         c.hostConfig.CPUQuota,
		BlkioWeight:      c.hostConfig.BlkioWeight,
		BlkioReadLimit:   constructBlkioArgs(c.hostConfig.Binds, c.hostConfig.BlkioReadLimit),
		Rlimits:          rlimits,
		OomKillDisable:   c.hostConfig.OomKillDisable,
		MemorySwappiness: -1,
	}

	if c.hostConfig.MemorySwappiness != nil {
		resources.MemorySwappiness = *c.hostConfig.MemorySwappiness
	}

	processConfig := execdriver.ProcessConfig{
		Privileged: c.hostConfig.Privileged,
		Entrypoint: c.Path,
		Arguments:  c.Args,
		Tty:        c.Config.Tty,
		User:       c.Config.User,
	}

	processConfig.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	processConfig.Env = env

	c.command = &execdriver.Command{
		ID:                 c.ID,
		Rootfs:             c.rootfsPath(),
		ReadonlyRootfs:     c.hostConfig.ReadonlyRootfs,
		InitPath:           "/.dockerinit",
		WorkingDir:         c.Config.WorkingDir,
		Network:            en,
		Ipc:                ipc,
		Pid:                pid,
		UTS:                uts,
		Resources:          resources,
		AllowedDevices:     allowedDevices,
		AutoCreatedDevices: autoCreatedDevices,
		CapAdd:             c.hostConfig.CapAdd.Slice(),
		CapDrop:            c.hostConfig.CapDrop.Slice(),
		GroupAdd:           c.hostConfig.GroupAdd,
		ProcessConfig:      processConfig,
		ProcessLabel:       c.getProcessLabel(),
		MountLabel:         c.getMountLabel(),
		LxcConfig:          lxcConfig,
		AppArmorProfile:    c.AppArmorProfile,
		CgroupParent:       c.hostConfig.CgroupParent,
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

// GetSize returns the real size & virtual size of the container.
func (container *Container) getSize() (int64, int64) {
	var (
		sizeRw, sizeRootfs int64
		err                error
		driver             = container.daemon.driver
	)

	if err := container.Mount(); err != nil {
		logrus.Errorf("Failed to compute size of container rootfs %s: %s", container.ID, err)
		return sizeRw, sizeRootfs
	}
	defer container.Unmount()

	initID := fmt.Sprintf("%s-init", container.ID)
	sizeRw, err = driver.DiffSize(container.ID, initID)
	if err != nil {
		logrus.Errorf("Driver %s couldn't return diff size of container %s: %s", driver, container.ID, err)
		// FIXME: GetSize should return an error. Not changing it now in case
		// there is a side-effect.
		sizeRw = -1
	}

	if _, err = os.Stat(container.basefs); err == nil {
		if sizeRootfs, err = directory.Size(container.basefs); err != nil {
			sizeRootfs = -1
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

func (container *Container) buildSandboxOptions() ([]libnetwork.SandboxOption, error) {
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
	} else if container.daemon.execDriver.SupportsHooks() {
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
	} else if len(container.daemon.configStore.DNS) > 0 {
		dns = container.daemon.configStore.DNS
	}

	for _, d := range dns {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNS(d))
	}

	if len(container.hostConfig.DNSSearch) > 0 {
		dnsSearch = container.hostConfig.DNSSearch
	} else if len(container.daemon.configStore.DNSSearch) > 0 {
		dnsSearch = container.daemon.configStore.DNSSearch
	}

	for _, ds := range dnsSearch {
		sboxOptions = append(sboxOptions, libnetwork.OptionDNSSearch(ds))
	}

	if len(container.hostConfig.DNSOptions) > 0 {
		dnsOptions = container.hostConfig.DNSOptions
	} else if len(container.daemon.configStore.DNSOptions) > 0 {
		dnsOptions = container.daemon.configStore.DNSOptions
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

	var childEndpoints, parentEndpoints []string

	children, err := container.daemon.children(container.Name)
	if err != nil {
		return nil, err
	}

	for linkAlias, child := range children {
		_, alias := path.Split(linkAlias)
		// allow access to the linked container via the alias, real name, and container hostname
		aliasList := alias + " " + child.Config.Hostname
		// only add the name if alias isn't equal to the name
		if alias != child.Name[1:] {
			aliasList = aliasList + " " + child.Name[1:]
		}
		sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(aliasList, child.NetworkSettings.IPAddress))
		if child.NetworkSettings.EndpointID != "" {
			childEndpoints = append(childEndpoints, child.NetworkSettings.EndpointID)
		}
	}

	for _, extraHost := range container.hostConfig.ExtraHosts {
		// allow IPv6 addresses in extra hosts; only split on first ":"
		parts := strings.SplitN(extraHost, ":", 2)
		sboxOptions = append(sboxOptions, libnetwork.OptionExtraHost(parts[0], parts[1]))
	}

	refs := container.daemon.containerGraph().RefPaths(container.ID)
	for _, ref := range refs {
		if ref.ParentID == "0" {
			continue
		}

		c, err := container.daemon.Get(ref.ParentID)
		if err != nil {
			logrus.Error(err)
		}

		if c != nil && !container.daemon.configStore.DisableBridge && container.hostConfig.NetworkMode.IsPrivate() {
			logrus.Debugf("Update /etc/hosts of %s for alias %s with ip %s", c.ID, ref.Name, container.NetworkSettings.IPAddress)
			sboxOptions = append(sboxOptions, libnetwork.OptionParentUpdate(c.ID, ref.Name, container.NetworkSettings.IPAddress))
			if c.NetworkSettings.EndpointID != "" {
				parentEndpoints = append(parentEndpoints, c.NetworkSettings.EndpointID)
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

	if mac, ok := driverInfo[netlabel.MacAddress]; ok {
		networkSettings.MacAddress = mac.(net.HardwareAddr).String()
	}

	networkSettings.Ports = nat.PortMap{}

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

func (container *Container) buildEndpointInfo(ep libnetwork.Endpoint, networkSettings *network.Settings) (*network.Settings, error) {
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

	iface := epInfo.Iface()
	if iface == nil {
		return networkSettings, nil
	}

	ones, _ := iface.Address().Mask.Size()
	networkSettings.IPAddress = iface.Address().IP.String()
	networkSettings.IPPrefixLen = ones

	if iface.AddressIPv6().IP.To16() != nil {
		onesv6, _ := iface.AddressIPv6().Mask.Size()
		networkSettings.GlobalIPv6Address = iface.AddressIPv6().IP.String()
		networkSettings.GlobalIPv6PrefixLen = onesv6
	}

	return networkSettings, nil
}

func (container *Container) updateJoinInfo(ep libnetwork.Endpoint) error {
	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return nil
	}

	container.NetworkSettings.Gateway = epInfo.Gateway().String()
	if epInfo.GatewayIPv6().To16() != nil {
		container.NetworkSettings.IPv6Gateway = epInfo.GatewayIPv6().String()
	}

	return nil
}

func (container *Container) updateEndpointNetworkSettings(n libnetwork.Network, ep libnetwork.Endpoint) error {
	networkSettings := &network.Settings{NetworkID: n.ID(), EndpointID: ep.ID()}

	networkSettings, err := container.buildPortMapInfo(ep, networkSettings)
	if err != nil {
		return err
	}

	networkSettings, err = container.buildEndpointInfo(ep, networkSettings)
	if err != nil {
		return err
	}

	if container.hostConfig.NetworkMode == runconfig.NetworkMode("bridge") {
		networkSettings.Bridge = container.daemon.configStore.Bridge.Iface
	}

	container.NetworkSettings = networkSettings
	return nil
}

func (container *Container) updateSandboxNetworkSettings(sb libnetwork.Sandbox) error {
	container.NetworkSettings.SandboxID = sb.ID()
	container.NetworkSettings.SandboxKey = sb.Key()
	return nil
}

// UpdateNetwork is used to update the container's network (e.g. when linked containers
// get removed/unlinked).
func (container *Container) updateNetwork() error {
	ctrl := container.daemon.netController
	sid := container.NetworkSettings.SandboxID

	sb, err := ctrl.SandboxByID(sid)
	if err != nil {
		return derr.ErrorCodeNoSandbox.WithArgs(sid, err)
	}

	options, err := container.buildSandboxOptions()
	if err != nil {
		return derr.ErrorCodeNetworkUpdate.WithArgs(err)
	}

	if err := sb.Refresh(options...); err != nil {
		return derr.ErrorCodeNetworkRefresh.WithArgs(sid, err)
	}

	return nil
}

func (container *Container) buildCreateEndpointOptions() ([]libnetwork.EndpointOption, error) {
	var (
		portSpecs     = make(nat.PortSet)
		bindings      = make(nat.PortMap)
		pbList        []types.PortBinding
		exposeList    []types.TransportPort
		createOptions []libnetwork.EndpointOption
	)

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

func parseService(controller libnetwork.NetworkController, service string) (string, string, string) {
	dn := controller.Config().Daemon.DefaultNetwork
	dd := controller.Config().Daemon.DefaultDriver

	snd := strings.Split(service, ".")
	if len(snd) > 2 {
		return strings.Join(snd[:len(snd)-2], "."), snd[len(snd)-2], snd[len(snd)-1]
	}
	if len(snd) > 1 {
		return snd[0], snd[1], dd
	}
	return snd[0], dn, dd
}

func createNetwork(controller libnetwork.NetworkController, dnet string, driver string) (libnetwork.Network, error) {
	createOptions := []libnetwork.NetworkOption{}
	genericOption := options.Generic{}

	// Bridge driver is special due to legacy reasons
	if runconfig.NetworkMode(driver).IsBridge() {
		genericOption[netlabel.GenericData] = map[string]interface{}{
			"BridgeName":            dnet,
			"AllowNonDefaultBridge": "true",
		}
		networkOption := libnetwork.NetworkOptionGeneric(genericOption)
		createOptions = append(createOptions, networkOption)
	}

	return controller.NewNetwork(driver, dnet, createOptions...)
}

func (container *Container) secondaryNetworkRequired(primaryNetworkType string) bool {
	switch primaryNetworkType {
	case "bridge", "none", "host", "container":
		return false
	}

	if container.daemon.configStore.DisableBridge {
		return false
	}

	if container.Config.ExposedPorts != nil && len(container.Config.ExposedPorts) > 0 {
		return true
	}
	if container.hostConfig.PortBindings != nil && len(container.hostConfig.PortBindings) > 0 {
		return true
	}
	return false
}

func (container *Container) allocateNetwork() error {
	mode := container.hostConfig.NetworkMode
	controller := container.daemon.netController
	if container.Config.NetworkDisabled || mode.IsContainer() {
		return nil
	}

	networkDriver := string(mode)
	service := container.Config.PublishService
	networkName := mode.NetworkName()
	if mode.IsDefault() {
		if service != "" {
			service, networkName, networkDriver = parseService(controller, service)
		} else {
			networkName = controller.Config().Daemon.DefaultNetwork
			networkDriver = controller.Config().Daemon.DefaultDriver
		}
	} else if service != "" {
		return derr.ErrorCodeNetworkConflict
	}

	if runconfig.NetworkMode(networkDriver).IsBridge() && container.daemon.configStore.DisableBridge {
		container.Config.NetworkDisabled = true
		return nil
	}

	if service == "" {
		// dot character "." has a special meaning to support SERVICE[.NETWORK] format.
		// For backward compatibility, replacing "." with "-", instead of failing
		service = strings.Replace(container.Name, ".", "-", -1)
		// Service names dont like "/" in them. removing it instead of failing for backward compatibility
		service = strings.Replace(service, "/", "", -1)
	}

	if container.secondaryNetworkRequired(networkDriver) {
		// Configure Bridge as secondary network for port binding purposes
		if err := container.configureNetwork("bridge", service, "bridge", false); err != nil {
			return err
		}
	}

	if err := container.configureNetwork(networkName, service, networkDriver, mode.IsDefault()); err != nil {
		return err
	}

	return container.writeHostConfig()
}

func (container *Container) configureNetwork(networkName, service, networkDriver string, canCreateNetwork bool) error {
	controller := container.daemon.netController

	n, err := controller.NetworkByName(networkName)
	if err != nil {
		if _, ok := err.(libnetwork.ErrNoSuchNetwork); !ok || !canCreateNetwork {
			return err
		}

		if n, err = createNetwork(controller, networkName, networkDriver); err != nil {
			return err
		}
	}

	ep, err := n.EndpointByName(service)
	if err != nil {
		if _, ok := err.(libnetwork.ErrNoSuchEndpoint); !ok {
			return err
		}

		createOptions, err := container.buildCreateEndpointOptions()
		if err != nil {
			return err
		}

		ep, err = n.CreateEndpoint(service, createOptions...)
		if err != nil {
			return err
		}
	}

	if err := container.updateEndpointNetworkSettings(n, ep); err != nil {
		return err
	}

	var sb libnetwork.Sandbox
	controller.WalkSandboxes(func(s libnetwork.Sandbox) bool {
		if s.ContainerID() == container.ID {
			sb = s
			return true
		}
		return false
	})
	if sb == nil {
		options, err := container.buildSandboxOptions()
		if err != nil {
			return err
		}
		sb, err = controller.NewSandbox(container.ID, options...)
		if err != nil {
			return err
		}
	}

	container.updateSandboxNetworkSettings(sb)

	if err := ep.Join(sb); err != nil {
		return err
	}

	if err := container.updateJoinInfo(ep); err != nil {
		return derr.ErrorCodeJoinInfo.WithArgs(err)
	}

	return nil
}

func (container *Container) initializeNetworking() error {
	var err error

	if container.hostConfig.NetworkMode.IsContainer() {
		// we need to get the hosts files from the container to join
		nc, err := container.getNetworkedContainer()
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

	if err := container.allocateNetwork(); err != nil {
		return err
	}

	return container.buildHostnameFile()
}

// called from the libcontainer pre-start hook to set the network
// namespace configuration linkage to the libnetwork "sandbox" entity
func (container *Container) setNetworkNamespaceKey(pid int) error {
	path := fmt.Sprintf("/proc/%d/ns/net", pid)
	var sandbox libnetwork.Sandbox
	search := libnetwork.SandboxContainerWalker(&sandbox, container.ID)
	container.daemon.netController.WalkSandboxes(search)
	if sandbox == nil {
		return fmt.Errorf("no sandbox present for %s", container.ID)
	}

	return sandbox.SetKey(path)
}

func (container *Container) getIpcContainer() (*Container, error) {
	containerID := container.hostConfig.IpcMode.Container()
	c, err := container.daemon.Get(containerID)
	if err != nil {
		return nil, err
	}
	if !c.IsRunning() {
		return nil, derr.ErrorCodeIPCRunning
	}
	return c, nil
}

func (container *Container) setupWorkingDirectory() error {
	if container.Config.WorkingDir != "" {
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
	}
	return nil
}

func (container *Container) getNetworkedContainer() (*Container, error) {
	parts := strings.SplitN(string(container.hostConfig.NetworkMode), ":", 2)
	switch parts[0] {
	case "container":
		if len(parts) != 2 {
			return nil, derr.ErrorCodeParseContainer
		}
		nc, err := container.daemon.Get(parts[1])
		if err != nil {
			return nil, err
		}
		if container == nc {
			return nil, derr.ErrorCodeJoinSelf
		}
		if !nc.IsRunning() {
			return nil, derr.ErrorCodeJoinRunning.WithArgs(parts[1])
		}
		return nc, nil
	default:
		return nil, derr.ErrorCodeModeNotContainer
	}
}

func (container *Container) releaseNetwork() {
	if container.hostConfig.NetworkMode.IsContainer() || container.Config.NetworkDisabled {
		return
	}

	sid := container.NetworkSettings.SandboxID
	eid := container.NetworkSettings.EndpointID
	nid := container.NetworkSettings.NetworkID

	container.NetworkSettings = &network.Settings{}

	if sid == "" || nid == "" || eid == "" {
		return
	}

	sb, err := container.daemon.netController.SandboxByID(sid)
	if err != nil {
		logrus.Errorf("error locating sandbox id %s: %v", sid, err)
		return
	}

	n, err := container.daemon.netController.NetworkByID(nid)
	if err != nil {
		logrus.Errorf("error locating network id %s: %v", nid, err)
		return
	}

	ep, err := n.EndpointByID(eid)
	if err != nil {
		logrus.Errorf("error locating endpoint id %s: %v", eid, err)
		return
	}

	if err := sb.Delete(); err != nil {
		logrus.Errorf("Error deleting sandbox id %s for container %s: %v", sid, container.ID, err)
		return
	}

	// In addition to leaving all endpoints, delete implicitly created endpoint
	if container.Config.PublishService == "" {
		if err := ep.Delete(); err != nil {
			logrus.Errorf("deleting endpoint failed: %v", err)
		}
	}
}

func (container *Container) unmountVolumes(forceSyscall bool) error {
	var volumeMounts []mountPoint

	for _, mntPoint := range container.MountPoints {
		dest, err := container.GetResourcePath(mntPoint.Destination)
		if err != nil {
			return err
		}

		volumeMounts = append(volumeMounts, mountPoint{Destination: dest, Volume: mntPoint.Volume})
	}

	for _, mnt := range container.networkMounts() {
		dest, err := container.GetResourcePath(mnt.Destination)
		if err != nil {
			return err
		}

		volumeMounts = append(volumeMounts, mountPoint{Destination: dest})
	}

	for _, volumeMount := range volumeMounts {
		if forceSyscall {
			syscall.Unmount(volumeMount.Destination, 0)
		}

		if volumeMount.Volume != nil {
			if err := volumeMount.Volume.Unmount(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (container *Container) networkMounts() []execdriver.Mount {
	var mounts []execdriver.Mount
	shared := container.hostConfig.NetworkMode.IsContainer()
	if container.ResolvConfPath != "" {
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
	if container.HostnamePath != "" {
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
	if container.HostsPath != "" {
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
	return mounts
}

func (container *Container) addBindMountPoint(name, source, destination string, rw bool) {
	container.MountPoints[destination] = &mountPoint{
		Name:        name,
		Source:      source,
		Destination: destination,
		RW:          rw,
	}
}

func (container *Container) addLocalMountPoint(name, destination string, rw bool) {
	container.MountPoints[destination] = &mountPoint{
		Name:        name,
		Driver:      volume.DefaultDriverName,
		Destination: destination,
		RW:          rw,
	}
}

func (container *Container) addMountPointWithVolume(destination string, vol volume.Volume, rw bool) {
	container.MountPoints[destination] = &mountPoint{
		Name:        vol.Name(),
		Driver:      vol.DriverName(),
		Destination: destination,
		RW:          rw,
		Volume:      vol,
	}
}

func (container *Container) isDestinationMounted(destination string) bool {
	return container.MountPoints[destination] != nil
}

func (container *Container) prepareMountPoints() error {
	for _, config := range container.MountPoints {
		if len(config.Driver) > 0 {
			v, err := container.daemon.createVolume(config.Name, config.Driver, nil)
			if err != nil {
				return err
			}
			config.Volume = v
		}
	}
	return nil
}

func (container *Container) removeMountPoints(rm bool) error {
	var rmErrors []string
	for _, m := range container.MountPoints {
		if m.Volume == nil {
			continue
		}
		container.daemon.volumes.Decrement(m.Volume)
		if rm {
			err := container.daemon.volumes.Remove(m.Volume)
			// ErrVolumeInUse is ignored because having this
			// volume being referenced by othe container is
			// not an error, but an implementation detail.
			// This prevents docker from logging "ERROR: Volume in use"
			// where there is another container using the volume.
			if err != nil && err != ErrVolumeInUse {
				rmErrors = append(rmErrors, err.Error())
			}
		}
	}
	if len(rmErrors) > 0 {
		return derr.ErrorCodeRemovingVolume.WithArgs(strings.Join(rmErrors, "\n"))
	}
	return nil
}
