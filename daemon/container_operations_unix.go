//go:build linux || freebsd
// +build linux freebsd

package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/links"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/runconfig"
	"github.com/moby/sys/mount"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

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

func (daemon *Daemon) getIpcContainer(id string) (*container.Container, error) {
	errMsg := "can't join IPC of container " + id
	// Check the container exists
	ctr, err := daemon.GetContainer(id)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}
	// Check the container is running and not restarting
	if err := daemon.checkContainer(ctr, containerIsRunning, containerIsNotRestarting); err != nil {
		return nil, errors.Wrap(err, errMsg)
	}
	// Check the container ipc is shareable
	if st, err := os.Stat(ctr.ShmPath); err != nil || !st.IsDir() {
		if err == nil || os.IsNotExist(err) {
			return nil, errors.New(errMsg + ": non-shareable IPC (hint: use IpcMode:shareable for the donor container)")
		}
		// stat() failed?
		return nil, errors.Wrap(err, errMsg+": unexpected error from stat "+ctr.ShmPath)
	}

	return ctr, nil
}

func (daemon *Daemon) getPidContainer(ctr *container.Container) (*container.Container, error) {
	containerID := ctr.HostConfig.PidMode.Container()
	ctr, err := daemon.GetContainer(containerID)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot join PID of a non running container: %s", containerID)
	}
	return ctr, daemon.checkContainer(ctr, containerIsRunning, containerIsNotRestarting)
}

func containerIsRunning(c *container.Container) error {
	if !c.IsRunning() {
		return errdefs.Conflict(errors.Errorf("container %s is not running", c.ID))
	}
	return nil
}

func containerIsNotRestarting(c *container.Container) error {
	if c.IsRestarting() {
		return errContainerIsRestarting(c.ID)
	}
	return nil
}

func (daemon *Daemon) setupIpcDirs(c *container.Container) error {
	ipcMode := c.HostConfig.IpcMode

	switch {
	case ipcMode.IsContainer():
		ic, err := daemon.getIpcContainer(ipcMode.Container())
		if err != nil {
			return err
		}
		c.ShmPath = ic.ShmPath

	case ipcMode.IsHost():
		if _, err := os.Stat("/dev/shm"); err != nil {
			return fmt.Errorf("/dev/shm is not mounted, but must be for --ipc=host")
		}
		c.ShmPath = "/dev/shm"

	case ipcMode.IsPrivate(), ipcMode.IsNone():
		// c.ShmPath will/should not be used, so make it empty.
		// Container's /dev/shm mount comes from OCI spec.
		c.ShmPath = ""

	case ipcMode.IsEmpty():
		// A container was created by an older version of the daemon.
		// The default behavior used to be what is now called "shareable".
		fallthrough

	case ipcMode.IsShareable():
		rootIDs := daemon.idMapping.RootPair()
		if !c.HasMountFor("/dev/shm") {
			shmPath, err := c.ShmResourcePath()
			if err != nil {
				return err
			}

			if err := idtools.MkdirAllAndChown(shmPath, 0700, rootIDs); err != nil {
				return err
			}

			shmproperty := "mode=1777,size=" + strconv.FormatInt(c.HostConfig.ShmSize, 10)
			if err := unix.Mount("shm", shmPath, "tmpfs", uintptr(unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV), label.FormatMountLabel(shmproperty, c.GetMountLabel())); err != nil {
				return fmt.Errorf("mounting shm tmpfs: %s", err)
			}
			if err := os.Chown(shmPath, rootIDs.UID, rootIDs.GID); err != nil {
				return err
			}
			c.ShmPath = shmPath
		}

	default:
		return fmt.Errorf("invalid IPC mode: %v", ipcMode)
	}

	return nil
}

func (daemon *Daemon) setupSecretDir(c *container.Container) (setupErr error) {
	if len(c.SecretReferences) == 0 && len(c.ConfigReferences) == 0 {
		return nil
	}

	if err := daemon.createSecretsDir(c); err != nil {
		return err
	}
	defer func() {
		if setupErr != nil {
			daemon.cleanupSecretDir(c)
		}
	}()

	if c.DependencyStore == nil {
		return fmt.Errorf("secret store is not initialized")
	}

	// retrieve possible remapped range start for root UID, GID
	rootIDs := daemon.idMapping.RootPair()

	for _, s := range c.SecretReferences {
		// TODO (ehazlett): use type switch when more are supported
		if s.File == nil {
			logrus.Error("secret target type is not a file target")
			continue
		}

		// secrets are created in the SecretMountPath on the host, at a
		// single level
		fPath, err := c.SecretFilePath(*s)
		if err != nil {
			return errors.Wrap(err, "error getting secret file path")
		}
		if err := idtools.MkdirAllAndChown(filepath.Dir(fPath), 0700, rootIDs); err != nil {
			return errors.Wrap(err, "error creating secret mount path")
		}

		logrus.WithFields(logrus.Fields{
			"name": s.File.Name,
			"path": fPath,
		}).Debug("injecting secret")
		secret, err := c.DependencyStore.Secrets().Get(s.SecretID)
		if err != nil {
			return errors.Wrap(err, "unable to get secret from secret store")
		}
		if err := os.WriteFile(fPath, secret.Spec.Data, s.File.Mode); err != nil {
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

		if err := os.Chown(fPath, rootIDs.UID+uid, rootIDs.GID+gid); err != nil {
			return errors.Wrap(err, "error setting ownership for secret")
		}
		if err := os.Chmod(fPath, s.File.Mode); err != nil {
			return errors.Wrap(err, "error setting file mode for secret")
		}
	}

	for _, configRef := range c.ConfigReferences {
		// TODO (ehazlett): use type switch when more are supported
		if configRef.File == nil {
			// Runtime configs are not mounted into the container, but they're
			// a valid type of config so we should not error when we encounter
			// one.
			if configRef.Runtime == nil {
				logrus.Error("config target type is not a file or runtime target")
			}
			// However, in any case, this isn't a file config, so we have no
			// further work to do
			continue
		}

		fPath, err := c.ConfigFilePath(*configRef)
		if err != nil {
			return errors.Wrap(err, "error getting config file path for container")
		}
		if err := idtools.MkdirAllAndChown(filepath.Dir(fPath), 0700, rootIDs); err != nil {
			return errors.Wrap(err, "error creating config mount path")
		}

		logrus.WithFields(logrus.Fields{
			"name": configRef.File.Name,
			"path": fPath,
		}).Debug("injecting config")
		config, err := c.DependencyStore.Configs().Get(configRef.ConfigID)
		if err != nil {
			return errors.Wrap(err, "unable to get config from config store")
		}
		if err := os.WriteFile(fPath, config.Spec.Data, configRef.File.Mode); err != nil {
			return errors.Wrap(err, "error injecting config")
		}

		uid, err := strconv.Atoi(configRef.File.UID)
		if err != nil {
			return err
		}
		gid, err := strconv.Atoi(configRef.File.GID)
		if err != nil {
			return err
		}

		if err := os.Chown(fPath, rootIDs.UID+uid, rootIDs.GID+gid); err != nil {
			return errors.Wrap(err, "error setting ownership for config")
		}
		if err := os.Chmod(fPath, configRef.File.Mode); err != nil {
			return errors.Wrap(err, "error setting file mode for config")
		}
	}

	return daemon.remountSecretDir(c)
}

// createSecretsDir is used to create a dir suitable for storing container secrets.
// In practice this is using a tmpfs mount and is used for both "configs" and "secrets"
func (daemon *Daemon) createSecretsDir(c *container.Container) error {
	// retrieve possible remapped range start for root UID, GID
	rootIDs := daemon.idMapping.RootPair()
	dir, err := c.SecretMountPath()
	if err != nil {
		return errors.Wrap(err, "error getting container secrets dir")
	}

	// create tmpfs
	if err := idtools.MkdirAllAndChown(dir, 0700, rootIDs); err != nil {
		return errors.Wrap(err, "error creating secret local mount path")
	}

	tmpfsOwnership := fmt.Sprintf("uid=%d,gid=%d", rootIDs.UID, rootIDs.GID)
	if err := mount.Mount("tmpfs", dir, "tmpfs", "nodev,nosuid,noexec,"+tmpfsOwnership); err != nil {
		return errors.Wrap(err, "unable to setup secret mount")
	}
	return nil
}

func (daemon *Daemon) remountSecretDir(c *container.Container) error {
	dir, err := c.SecretMountPath()
	if err != nil {
		return errors.Wrap(err, "error getting container secrets path")
	}
	if err := label.Relabel(dir, c.MountLabel, false); err != nil {
		logrus.WithError(err).WithField("dir", dir).Warn("Error while attempting to set selinux label")
	}
	rootIDs := daemon.idMapping.RootPair()
	tmpfsOwnership := fmt.Sprintf("uid=%d,gid=%d", rootIDs.UID, rootIDs.GID)

	// remount secrets ro
	if err := mount.Mount("tmpfs", dir, "tmpfs", "remount,ro,"+tmpfsOwnership); err != nil {
		return errors.Wrap(err, "unable to remount dir as readonly")
	}

	return nil
}

func (daemon *Daemon) cleanupSecretDir(c *container.Container) {
	dir, err := c.SecretMountPath()
	if err != nil {
		logrus.WithError(err).WithField("container", c.ID).Warn("error getting secrets mount path for container")
	}
	if err := mount.RecursiveUnmount(dir); err != nil {
		logrus.WithField("dir", dir).WithError(err).Warn("Error while attempting to unmount dir, this may prevent removal of container.")
	}
	if err := os.RemoveAll(dir); err != nil {
		logrus.WithField("dir", dir).WithError(err).Error("Error removing dir.")
	}
}

func killProcessDirectly(container *container.Container) error {
	pid := container.GetPID()
	if pid == 0 {
		// Ensure that we don't kill ourselves
		return nil
	}

	if err := unix.Kill(pid, syscall.SIGKILL); err != nil {
		if err != unix.ESRCH {
			return errdefs.System(err)
		}
		err = errNoSuchProcess{pid, syscall.SIGKILL}
		logrus.WithError(err).WithField("container", container.ID).Debug("no such process")
		return err
	}

	// In case there were some exceptions(e.g., state of zombie and D)
	if system.IsProcessAlive(pid) {
		// Since we can not kill a zombie pid, add zombie check here
		isZombie, err := system.IsProcessZombie(pid)
		// TODO(thaJeztah) should we ignore os.IsNotExist() here? ("/proc/<pid>/stat" will be gone if the process exited)
		if err != nil {
			logrus.WithError(err).WithField("container", container.ID).Warn("Container state is invalid")
			return err
		}
		if isZombie {
			return errdefs.System(errors.Errorf("container %s PID %d is zombie and can not be killed. Use the --init option when creating containers to run an init inside the container that forwards signals and reaps processes", stringid.TruncateID(container.ID), pid))
		}
	}
	return nil
}

func isLinkable(child *container.Container) bool {
	// A container is linkable only if it belongs to the default network
	_, ok := child.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
	return ok
}

func enableIPOnPredefinedNetwork() bool {
	return false
}

// serviceDiscoveryOnDefaultNetwork indicates if service discovery is supported on the default network
func serviceDiscoveryOnDefaultNetwork() bool {
	return false
}

func (daemon *Daemon) setupPathsAndSandboxOptions(container *container.Container, sboxOptions *[]libnetwork.SandboxOption) error {
	var err error

	// Set the correct paths for /etc/hosts and /etc/resolv.conf, based on the
	// networking-mode of the container. Note that containers with "container"
	// networking are already handled in "initializeNetworking()" before we reach
	// this function, so do not have to be accounted for here.
	switch {
	case container.HostConfig.NetworkMode.IsHost():
		// In host-mode networking, the container does not have its own networking
		// namespace, so both `/etc/hosts` and `/etc/resolv.conf` should be the same
		// as on the host itself. The container gets a copy of these files.
		*sboxOptions = append(
			*sboxOptions,
			libnetwork.OptionOriginHostsPath("/etc/hosts"),
			libnetwork.OptionOriginResolvConfPath("/etc/resolv.conf"),
		)
	case container.HostConfig.NetworkMode.IsUserDefined():
		// The container uses a user-defined network. We use the embedded DNS
		// server for container name resolution and to act as a DNS forwarder
		// for external DNS resolution.
		// We parse the DNS server(s) that are defined in /etc/resolv.conf on
		// the host, which may be a local DNS server (for example, if DNSMasq or
		// systemd-resolvd are in use). The embedded DNS server forwards DNS
		// resolution to the DNS server configured on the host, which in itself
		// may act as a forwarder for external DNS servers.
		// If systemd-resolvd is used, the "upstream" DNS servers can be found in
		// /run/systemd/resolve/resolv.conf. We do not query those DNS servers
		// directly, as they can be dynamically reconfigured.
		*sboxOptions = append(
			*sboxOptions,
			libnetwork.OptionOriginResolvConfPath("/etc/resolv.conf"),
		)
	default:
		// For other situations, such as the default bridge network, container
		// discovery / name resolution is handled through /etc/hosts, and no
		// embedded DNS server is available. Without the embedded DNS, we
		// cannot use local DNS servers on the host (for example, if DNSMasq or
		// systemd-resolvd is used). If systemd-resolvd is used, we try to
		// determine the external DNS servers that are used on the host.
		// This situation is not ideal, because DNS servers configured in the
		// container are not updated after the container is created, but the
		// DNS servers on the host can be dynamically updated.
		//
		// Copy the host's resolv.conf for the container (/run/systemd/resolve/resolv.conf or /etc/resolv.conf)
		*sboxOptions = append(
			*sboxOptions,
			libnetwork.OptionOriginResolvConfPath(daemon.configStore.GetResolvConf()),
		)
	}

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

func (daemon *Daemon) initializeNetworkingPaths(container *container.Container, nc *container.Container) error {
	container.HostnamePath = nc.HostnamePath
	container.HostsPath = nc.HostsPath
	container.ResolvConfPath = nc.ResolvConfPath
	return nil
}

func (daemon *Daemon) setupContainerMountsRoot(c *container.Container) error {
	// get the root mount path so we can make it unbindable
	p, err := c.MountsResourcePath("")
	if err != nil {
		return err
	}
	return idtools.MkdirAllAndChown(p, 0710, idtools.Identity{UID: idtools.CurrentIdentity().UID, GID: daemon.IdentityMapping().RootPair().GID})
}
