// +build linux freebsd

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/links"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	containertypes "github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/docker/go-units"
	"github.com/docker/libnetwork"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/label"
)

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	var env []string
	children := daemon.children(container)

	bridgeSettings := container.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
	if bridgeSettings == nil {
		return nil, nil
	}

	for linkAlias, child := range children {
		if !child.IsRunning() {
			return nil, fmt.Errorf("Cannot link to a non running container: %s AS %s", child.Name, linkAlias)
		}

		childBridgeSettings := child.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
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
		PidsLimit:                    c.HostConfig.PidsLimit,
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
		NoNewPrivileges:    c.NoNewPrivileges,
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

// ConnectToNetwork connects a container to a network
func (daemon *Daemon) ConnectToNetwork(container *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings) error {
	if !container.Running {
		if container.RemovalInProgress || container.Dead {
			return errRemovalContainer(container.ID)
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

// DisconnectFromNetwork disconnects container from network n.
func (daemon *Daemon) DisconnectFromNetwork(container *container.Container, n libnetwork.Network, force bool) error {
	if container.HostConfig.NetworkMode.IsHost() && containertypes.NetworkMode(n.Type()).IsHost() {
		return runconfig.ErrConflictHostNetwork
	}
	if !container.Running {
		if container.RemovalInProgress || container.Dead {
			return errRemovalContainer(container.ID)
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

// called from the libcontainer pre-start hook to set the network
// namespace configuration linkage to the libnetwork "sandbox" entity
func (daemon *Daemon) setNetworkNamespaceKey(containerID string, pid int) error {
	path := fmt.Sprintf("/proc/%d/ns/net", pid)
	var sandbox libnetwork.Sandbox
	search := libnetwork.SandboxContainerWalker(&sandbox, containerID)
	daemon.netController.WalkSandboxes(search)
	if sandbox == nil {
		return fmt.Errorf("error locating sandbox id %s: no sandbox found", containerID)
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
		return nil, fmt.Errorf("cannot join IPC of a non running container: %s", containerID)
	}
	if c.IsRestarting() {
		return nil, errContainerIsRestarting(container.ID)
	}
	return c, nil
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
				e := errNoSuchProcess{pid, 9}
				logrus.Debug(e)
				return e
			}
		}
	}
	return nil
}

func getDevicesFromPath(deviceMapping containertypes.DeviceMapping) (devs []*configs.Device, err error) {
	resolvedPathOnHost := deviceMapping.PathOnHost

	// check if it is a symbolic link
	if src, e := os.Lstat(deviceMapping.PathOnHost); e == nil && src.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkedPathOnHost, e := os.Readlink(deviceMapping.PathOnHost); e == nil {
			resolvedPathOnHost = linkedPathOnHost
		}
	}

	device, err := devices.DeviceFromPath(resolvedPathOnHost, deviceMapping.CgroupPermissions)
	// if there was no error, return the device
	if err == nil {
		device.Path = deviceMapping.PathInContainer
		return append(devs, device), nil
	}

	// if the device is not a device node
	// try to see if it's a directory holding many devices
	if err == devices.ErrNotADevice {

		// check if it is a directory
		if src, e := os.Stat(resolvedPathOnHost); e == nil && src.IsDir() {

			// mount the internal devices recursively
			filepath.Walk(resolvedPathOnHost, func(dpath string, f os.FileInfo, e error) error {
				childDevice, e := devices.DeviceFromPath(dpath, deviceMapping.CgroupPermissions)
				if e != nil {
					// ignore the device
					return nil
				}

				// add the device to userSpecified devices
				childDevice.Path = strings.Replace(dpath, resolvedPathOnHost, deviceMapping.PathInContainer, 1)
				devs = append(devs, childDevice)

				return nil
			})
		}
	}

	if len(devs) > 0 {
		return devs, nil
	}

	return devs, fmt.Errorf("error gathering device information while adding custom device %q: %s", deviceMapping.PathOnHost, err)
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
	_, ok := child.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
	return ok
}

func errRemovalContainer(containerID string) error {
	return fmt.Errorf("Container %s is marked for removal and cannot be connected or disconnected to the network", containerID)
}
