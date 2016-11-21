// +build linux freebsd

package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cloudflare/cfssl/log"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/links"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libnetwork"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/label"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func u32Ptr(i int64) *uint32     { u := uint32(i); return &u }
func fmPtr(i int64) *os.FileMode { fm := os.FileMode(i); return &fm }

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	var env []string
	children := daemon.children(container)

	bridgeSettings := container.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
	if bridgeSettings == nil || bridgeSettings.EndpointSettings == nil {
		return nil, nil
	}

	for linkAlias, child := range children {
		if !child.IsRunning() {
			return nil, fmt.Errorf("Cannot link to a non running container: %s AS %s", child.Name, linkAlias)
		}

		childBridgeSettings := child.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
		if childBridgeSettings == nil || childBridgeSettings.EndpointSettings == nil {
			return nil, fmt.Errorf("container %s not attached to default bridge network", child.ID)
		}

		link := links.NewLink(
			bridgeSettings.IPAddress,
			childBridgeSettings.IPAddress,
			linkAlias,
			child.Config.Env,
			child.Config.ExposedPorts,
		)

		env = append(env, link.ToEnv()...)
	}

	return env, nil
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

func (daemon *Daemon) getPidContainer(container *container.Container) (*container.Container, error) {
	containerID := container.HostConfig.PidMode.Container()
	c, err := daemon.GetContainer(containerID)
	if err != nil {
		return nil, err
	}
	if !c.IsRunning() {
		return nil, fmt.Errorf("cannot join PID of a non running container: %s", containerID)
	}
	if c.IsRestarting() {
		return nil, errContainerIsRestarting(container.ID)
	}
	return c, nil
}

func (daemon *Daemon) setupIpcDirs(c *container.Container) error {
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
		c.ShmPath = ic.ShmPath
	} else if c.HostConfig.IpcMode.IsHost() {
		if _, err := os.Stat("/dev/shm"); err != nil {
			return fmt.Errorf("/dev/shm is not mounted, but must be for --ipc=host")
		}
		c.ShmPath = "/dev/shm"
	} else {
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

	}

	return nil
}

func (daemon *Daemon) setupSecretDir(c *container.Container) (setupErr error) {
	if len(c.SecretReferences) == 0 {
		return nil
	}

	localMountPath := c.SecretMountPath()
	logrus.Debugf("secrets: setting up secret dir: %s", localMountPath)

	defer func() {
		if setupErr != nil {
			// cleanup
			_ = detachMounted(localMountPath)

			if err := os.RemoveAll(localMountPath); err != nil {
				log.Errorf("error cleaning up secret mount: %s", err)
			}
		}
	}()

	// retrieve possible remapped range start for root UID, GID
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	// create tmpfs
	if err := idtools.MkdirAllAs(localMountPath, 0700, rootUID, rootGID); err != nil {
		return errors.Wrap(err, "error creating secret local mount path")
	}
	tmpfsOwnership := fmt.Sprintf("uid=%d,gid=%d", rootUID, rootGID)
	if err := mount.Mount("tmpfs", localMountPath, "tmpfs", "nodev,nosuid,noexec,"+tmpfsOwnership); err != nil {
		return errors.Wrap(err, "unable to setup secret mount")
	}

	for _, s := range c.SecretReferences {
		if c.SecretStore == nil {
			return fmt.Errorf("secret store is not initialized")
		}

		// TODO (ehazlett): use type switch when more are supported
		if s.File == nil {
			return fmt.Errorf("secret target type is not a file target")
		}

		targetPath := filepath.Clean(s.File.Name)
		// ensure that the target is a filename only; no paths allowed
		if targetPath != filepath.Base(targetPath) {
			return fmt.Errorf("error creating secret: secret must not be a path")
		}

		fPath := filepath.Join(localMountPath, targetPath)
		if err := idtools.MkdirAllAs(filepath.Dir(fPath), 0700, rootUID, rootGID); err != nil {
			return errors.Wrap(err, "error creating secret mount path")
		}

		logrus.WithFields(logrus.Fields{
			"name": s.File.Name,
			"path": fPath,
		}).Debug("injecting secret")
		secret := c.SecretStore.Get(s.SecretID)
		if secret == nil {
			return fmt.Errorf("unable to get secret from secret store")
		}
		if err := ioutil.WriteFile(fPath, secret.Spec.Data, s.File.Mode); err != nil {
			return errors.Wrap(err, "error injecting secret")
		}

		uid, err := strconv.Atoi(s.File.UID)
		if err != nil {
			return err
		}
		gid, err := strconv.Atoi(s.File.GID)
		if err != nil {
			return err
		}

		if err := os.Chown(fPath, rootUID+uid, rootGID+gid); err != nil {
			return errors.Wrap(err, "error setting ownership for secret")
		}
	}

	// remount secrets ro
	if err := mount.Mount("tmpfs", localMountPath, "tmpfs", "remount,ro,"+tmpfsOwnership); err != nil {
		return errors.Wrap(err, "unable to remount secret dir as readonly")
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

func specDevice(d *configs.Device) specs.Device {
	return specs.Device{
		Type:     string(d.Type),
		Path:     d.Path,
		Major:    d.Major,
		Minor:    d.Minor,
		FileMode: fmPtr(int64(d.FileMode)),
		UID:      u32Ptr(int64(d.Uid)),
		GID:      u32Ptr(int64(d.Gid)),
	}
}

func specDeviceCgroup(d *configs.Device) specs.DeviceCgroup {
	t := string(d.Type)
	return specs.DeviceCgroup{
		Allow:  true,
		Type:   &t,
		Major:  &d.Major,
		Minor:  &d.Minor,
		Access: &d.Permissions,
	}
}

func getDevicesFromPath(deviceMapping containertypes.DeviceMapping) (devs []specs.Device, devPermissions []specs.DeviceCgroup, err error) {
	resolvedPathOnHost := deviceMapping.PathOnHost

	// check if it is a symbolic link
	if src, e := os.Lstat(deviceMapping.PathOnHost); e == nil && src.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkedPathOnHost, e := filepath.EvalSymlinks(deviceMapping.PathOnHost); e == nil {
			resolvedPathOnHost = linkedPathOnHost
		}
	}

	device, err := devices.DeviceFromPath(resolvedPathOnHost, deviceMapping.CgroupPermissions)
	// if there was no error, return the device
	if err == nil {
		device.Path = deviceMapping.PathInContainer
		return append(devs, specDevice(device)), append(devPermissions, specDeviceCgroup(device)), nil
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
				devs = append(devs, specDevice(childDevice))
				devPermissions = append(devPermissions, specDeviceCgroup(childDevice))

				return nil
			})
		}
	}

	if len(devs) > 0 {
		return devs, devPermissions, nil
	}

	return devs, devPermissions, fmt.Errorf("error gathering device information while adding custom device %q: %s", deviceMapping.PathOnHost, err)
}

func detachMounted(path string) error {
	return syscall.Unmount(path, syscall.MNT_DETACH)
}

func isLinkable(child *container.Container) bool {
	// A container is linkable only if it belongs to the default network
	_, ok := child.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
	return ok
}

func enableIPOnPredefinedNetwork() bool {
	return false
}

func (daemon *Daemon) isNetworkHotPluggable() bool {
	return true
}

func setupPathsAndSandboxOptions(container *container.Container, sboxOptions *[]libnetwork.SandboxOption) error {
	var err error

	container.HostsPath, err = container.GetRootResourcePath("hosts")
	if err != nil {
		return err
	}
	*sboxOptions = append(*sboxOptions, libnetwork.OptionHostsPath(container.HostsPath))

	container.ResolvConfPath, err = container.GetRootResourcePath("resolv.conf")
	if err != nil {
		return err
	}
	*sboxOptions = append(*sboxOptions, libnetwork.OptionResolvConfPath(container.ResolvConfPath))
	return nil
}

func initializeNetworkingPaths(container *container.Container, nc *container.Container) {
	container.HostnamePath = nc.HostnamePath
	container.HostsPath = nc.HostsPath
	container.ResolvConfPath = nc.ResolvConfPath
}
