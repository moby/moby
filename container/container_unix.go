// +build linux freebsd

package container

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/volume"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
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
	AppArmorProfile string
	HostnamePath    string
	HostsPath       string
	ShmPath         string
	MqueuePath      string
	ResolvConfPath  string
	SeccompProfile  string
}

// CreateDaemonEnvironment returns the list of all environment variables given the list of
// environment variables related to links.
// Sets PATH, HOSTNAME and if container.Config.Tty is set: TERM.
// The defaults set here do not override the values in container.Config.Env
func (container *Container) CreateDaemonEnvironment(linkedEnv []string) []string {
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

// TrySetNetworkMount attempts to set the network mounts given a provided destination and
// the path to use for it; return true if the given destination was a network mount file
func (container *Container) TrySetNetworkMount(destination string, path string) bool {
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

// BuildHostnameFile writes the container's hostname file.
func (container *Container) BuildHostnameFile() error {
	hostnamePath, err := container.GetRootResourcePath("hostname")
	if err != nil {
		return err
	}
	container.HostnamePath = hostnamePath

	if container.Config.Domainname != "" {
		return ioutil.WriteFile(container.HostnamePath, []byte(fmt.Sprintf("%s.%s\n", container.Config.Hostname, container.Config.Domainname)), 0644)
	}
	return ioutil.WriteFile(container.HostnamePath, []byte(container.Config.Hostname+"\n"), 0644)
}

// GetEndpointInNetwork returns the container's endpoint to the provided network.
func (container *Container) GetEndpointInNetwork(n libnetwork.Network) (libnetwork.Endpoint, error) {
	endpointName := strings.TrimPrefix(container.Name, "/")
	return n.EndpointByName(endpointName)
}

func (container *Container) buildPortMapInfo(ep libnetwork.Endpoint) error {
	if ep == nil {
		return derr.ErrorCodeEmptyEndpoint
	}

	networkSettings := container.NetworkSettings
	if networkSettings == nil {
		return derr.ErrorCodeEmptyNetwork
	}

	driverInfo, err := ep.DriverInfo()
	if err != nil {
		return err
	}

	if driverInfo == nil {
		// It is not an error for epInfo to be nil
		return nil
	}

	if networkSettings.Ports == nil {
		networkSettings.Ports = nat.PortMap{}
	}

	if expData, ok := driverInfo[netlabel.ExposedPorts]; ok {
		if exposedPorts, ok := expData.([]types.TransportPort); ok {
			for _, tp := range exposedPorts {
				natPort, err := nat.NewPort(tp.Proto.String(), strconv.Itoa(int(tp.Port)))
				if err != nil {
					return derr.ErrorCodeParsingPort.WithArgs(tp.Port, err)
				}
				networkSettings.Ports[natPort] = nil
			}
		}
	}

	mapData, ok := driverInfo[netlabel.PortMap]
	if !ok {
		return nil
	}

	if portMapping, ok := mapData.([]types.PortBinding); ok {
		for _, pp := range portMapping {
			natPort, err := nat.NewPort(pp.Proto.String(), strconv.Itoa(int(pp.Port)))
			if err != nil {
				return err
			}
			natBndg := nat.PortBinding{HostIP: pp.HostIP.String(), HostPort: strconv.Itoa(int(pp.HostPort))}
			networkSettings.Ports[natPort] = append(networkSettings.Ports[natPort], natBndg)
		}
	}

	return nil
}

// BuildEndpointInfo sets endpoint-related fields on container.NetworkSettings based on the provided network and endpoint.
func (container *Container) BuildEndpointInfo(n libnetwork.Network, ep libnetwork.Endpoint) error {
	if ep == nil {
		return derr.ErrorCodeEmptyEndpoint
	}

	networkSettings := container.NetworkSettings
	if networkSettings == nil {
		return derr.ErrorCodeEmptyNetwork
	}

	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return nil
	}

	if _, ok := networkSettings.Networks[n.Name()]; !ok {
		networkSettings.Networks[n.Name()] = new(network.EndpointSettings)
	}
	networkSettings.Networks[n.Name()].EndpointID = ep.ID()

	iface := epInfo.Iface()
	if iface == nil {
		return nil
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

	return nil
}

// UpdateJoinInfo updates network settings when container joins network n with endpoint ep.
func (container *Container) UpdateJoinInfo(n libnetwork.Network, ep libnetwork.Endpoint) error {
	if err := container.buildPortMapInfo(ep); err != nil {
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

// UpdateSandboxNetworkSettings updates the sandbox ID and Key.
func (container *Container) UpdateSandboxNetworkSettings(sb libnetwork.Sandbox) error {
	container.NetworkSettings.SandboxID = sb.ID()
	container.NetworkSettings.SandboxKey = sb.Key()
	return nil
}

// BuildCreateEndpointOptions builds endpoint options from a given network.
func (container *Container) BuildCreateEndpointOptions(n libnetwork.Network) ([]libnetwork.EndpointOption, error) {
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
	if n.Name() != container.HostConfig.NetworkMode.NetworkName() &&
		!(n.Name() == "bridge" && container.HostConfig.NetworkMode.IsDefault()) {
		return createOptions, nil
	}

	if container.Config.ExposedPorts != nil {
		portSpecs = container.Config.ExposedPorts
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

		if container.HostConfig.PublishAllPorts && len(binding) == 0 {
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

// SetupWorkingDirectory sets up the container's working directory as set in container.Config.WorkingDir
func (container *Container) SetupWorkingDirectory() error {
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

// appendNetworkMounts appends any network mounts to the array of mount points passed in
func appendNetworkMounts(container *Container, volumeMounts []volume.MountPoint) ([]volume.MountPoint, error) {
	for _, mnt := range container.NetworkMounts() {
		dest, err := container.GetResourcePath(mnt.Destination)
		if err != nil {
			return nil, err
		}
		volumeMounts = append(volumeMounts, volume.MountPoint{Destination: dest})
	}
	return volumeMounts, nil
}

// NetworkMounts returns the list of network mounts.
func (container *Container) NetworkMounts() []execdriver.Mount {
	var mounts []execdriver.Mount
	shared := container.HostConfig.NetworkMode.IsContainer()
	if container.ResolvConfPath != "" {
		if _, err := os.Stat(container.ResolvConfPath); err != nil {
			logrus.Warnf("ResolvConfPath set to %q, but can't stat this filename (err = %v); skipping", container.ResolvConfPath, err)
		} else {
			label.Relabel(container.ResolvConfPath, container.MountLabel, shared)
			writable := !container.HostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/resolv.conf"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.ResolvConfPath,
				Destination: "/etc/resolv.conf",
				Writable:    writable,
				Propagation: volume.DefaultPropagationMode,
			})
		}
	}
	if container.HostnamePath != "" {
		if _, err := os.Stat(container.HostnamePath); err != nil {
			logrus.Warnf("HostnamePath set to %q, but can't stat this filename (err = %v); skipping", container.HostnamePath, err)
		} else {
			label.Relabel(container.HostnamePath, container.MountLabel, shared)
			writable := !container.HostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/hostname"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.HostnamePath,
				Destination: "/etc/hostname",
				Writable:    writable,
				Propagation: volume.DefaultPropagationMode,
			})
		}
	}
	if container.HostsPath != "" {
		if _, err := os.Stat(container.HostsPath); err != nil {
			logrus.Warnf("HostsPath set to %q, but can't stat this filename (err = %v); skipping", container.HostsPath, err)
		} else {
			label.Relabel(container.HostsPath, container.MountLabel, shared)
			writable := !container.HostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/hosts"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.HostsPath,
				Destination: "/etc/hosts",
				Writable:    writable,
				Propagation: volume.DefaultPropagationMode,
			})
		}
	}
	return mounts
}

// CopyImagePathContent copies files in destination to the volume.
func (container *Container) CopyImagePathContent(v volume.Volume, destination string) error {
	rootfs, err := symlink.FollowSymlinkInScope(filepath.Join(container.BaseFS, destination), container.BaseFS)
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

// ShmResourcePath returns path to shm
func (container *Container) ShmResourcePath() (string, error) {
	return container.GetRootResourcePath("shm")
}

// MqueueResourcePath returns path to mqueue
func (container *Container) MqueueResourcePath() (string, error) {
	return container.GetRootResourcePath("mqueue")
}

// HasMountFor checks if path is a mountpoint
func (container *Container) HasMountFor(path string) bool {
	_, exists := container.MountPoints[path]
	return exists
}

// UnmountIpcMounts uses the provided unmount function to unmount shm and mqueue if they were mounted
func (container *Container) UnmountIpcMounts(unmount func(pth string) error) {
	if container.HostConfig.IpcMode.IsContainer() || container.HostConfig.IpcMode.IsHost() {
		return
	}

	var warnings []string

	if !container.HasMountFor("/dev/shm") {
		shmPath, err := container.ShmResourcePath()
		if err != nil {
			logrus.Error(err)
			warnings = append(warnings, err.Error())
		} else if shmPath != "" {
			if err := unmount(shmPath); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to umount %s: %v", shmPath, err))
			}

		}
	}

	if !container.HasMountFor("/dev/mqueue") {
		mqueuePath, err := container.MqueueResourcePath()
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

// IpcMounts returns the list of IPC mounts
func (container *Container) IpcMounts() []execdriver.Mount {
	var mounts []execdriver.Mount

	if !container.HasMountFor("/dev/shm") {
		label.SetFileLabel(container.ShmPath, container.MountLabel)
		mounts = append(mounts, execdriver.Mount{
			Source:      container.ShmPath,
			Destination: "/dev/shm",
			Writable:    true,
			Propagation: volume.DefaultPropagationMode,
		})
	}

	if !container.HasMountFor("/dev/mqueue") {
		label.SetFileLabel(container.MqueuePath, container.MountLabel)
		mounts = append(mounts, execdriver.Mount{
			Source:      container.MqueuePath,
			Destination: "/dev/mqueue",
			Writable:    true,
			Propagation: volume.DefaultPropagationMode,
		})
	}
	return mounts
}

func detachMounted(path string) error {
	return syscall.Unmount(path, syscall.MNT_DETACH)
}

// UnmountVolumes unmounts all volumes
func (container *Container) UnmountVolumes(forceSyscall bool) error {
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

// copyExistingContents copies from the source to the destination and
// ensures the ownership is appropriately set.
func copyExistingContents(source, destination string) error {
	volList, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	if len(volList) > 0 {
		srcList, err := ioutil.ReadDir(destination)
		if err != nil {
			return err
		}
		if len(srcList) == 0 {
			// If the source volume is empty copy files from the root into the volume
			if err := chrootarchive.CopyWithTar(source, destination); err != nil {
				return err
			}
		}
	}
	return copyOwnership(source, destination)
}

// copyOwnership copies the permissions and uid:gid of the source file
// to the destination file
func copyOwnership(source, destination string) error {
	stat, err := system.Stat(source)
	if err != nil {
		return err
	}

	if err := os.Chown(destination, int(stat.UID()), int(stat.GID())); err != nil {
		return err
	}

	return os.Chmod(destination, os.FileMode(stat.Mode()))
}

// TmpfsMounts returns the list of tmpfs mounts
func (container *Container) TmpfsMounts() []execdriver.Mount {
	var mounts []execdriver.Mount
	for dest, data := range container.HostConfig.Tmpfs {
		mounts = append(mounts, execdriver.Mount{
			Source:      "tmpfs",
			Destination: dest,
			Data:        data,
		})
	}
	return mounts
}
