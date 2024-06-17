//go:build linux || freebsd

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/links"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/process"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/moby/sys/mount"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func (daemon *Daemon) setupLinkedContainers(ctr *container.Container) ([]string, error) {
	var env []string
	children := daemon.children(ctr)

	bridgeSettings := ctr.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
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

func (daemon *Daemon) getIPCContainer(id string) (*container.Container, error) {
	// Check if the container exists, is running, and not restarting
	ctr, err := daemon.GetContainer(id)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}
	if !ctr.IsRunning() {
		return nil, errNotRunning(id)
	}
	if ctr.IsRestarting() {
		return nil, errContainerIsRestarting(id)
	}

	// Check the container ipc is shareable
	if st, err := os.Stat(ctr.ShmPath); err != nil || !st.IsDir() {
		if err == nil || os.IsNotExist(err) {
			return nil, errdefs.InvalidParameter(errors.New("container " + id + ": non-shareable IPC (hint: use IpcMode:shareable for the donor container)"))
		}
		// stat() failed?
		return nil, errdefs.System(errors.Wrap(err, "container "+id))
	}

	return ctr, nil
}

func (daemon *Daemon) getPIDContainer(id string) (*container.Container, error) {
	ctr, err := daemon.GetContainer(id)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}
	if !ctr.IsRunning() {
		return nil, errNotRunning(id)
	}
	if ctr.IsRestarting() {
		return nil, errContainerIsRestarting(id)
	}

	return ctr, nil
}

// setupContainerDirs sets up base container directories (root, ipc, tmpfs and secrets).
func (daemon *Daemon) setupContainerDirs(ctr *container.Container) (_ []container.Mount, err error) {
	if err := daemon.setupContainerMountsRoot(ctr); err != nil {
		return nil, err
	}

	if err := daemon.setupIPCDirs(ctr); err != nil {
		return nil, err
	}

	if err := daemon.setupSecretDir(ctr); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			daemon.cleanupSecretDir(ctr)
		}
	}()

	var ms []container.Mount
	if !ctr.HostConfig.IpcMode.IsPrivate() && !ctr.HostConfig.IpcMode.IsEmpty() {
		ms = append(ms, ctr.IpcMounts()...)
	}

	tmpfsMounts, err := ctr.TmpfsMounts()
	if err != nil {
		return nil, err
	}
	ms = append(ms, tmpfsMounts...)

	secretMounts, err := ctr.SecretMounts()
	if err != nil {
		return nil, err
	}
	ms = append(ms, secretMounts...)

	return ms, nil
}

func (daemon *Daemon) setupIPCDirs(ctr *container.Container) error {
	ipcMode := ctr.HostConfig.IpcMode

	switch {
	case ipcMode.IsContainer():
		ic, err := daemon.getIPCContainer(ipcMode.Container())
		if err != nil {
			return errors.Wrapf(err, "failed to join IPC namespace")
		}
		ctr.ShmPath = ic.ShmPath

	case ipcMode.IsHost():
		if _, err := os.Stat("/dev/shm"); err != nil {
			return fmt.Errorf("/dev/shm is not mounted, but must be for --ipc=host")
		}
		ctr.ShmPath = "/dev/shm"

	case ipcMode.IsPrivate(), ipcMode.IsNone():
		// c.ShmPath will/should not be used, so make it empty.
		// Container's /dev/shm mount comes from OCI spec.
		ctr.ShmPath = ""

	case ipcMode.IsEmpty():
		// A container was created by an older version of the daemon.
		// The default behavior used to be what is now called "shareable".
		fallthrough

	case ipcMode.IsShareable():
		rootIDs := daemon.idMapping.RootPair()
		if !ctr.HasMountFor("/dev/shm") {
			shmPath, err := ctr.ShmResourcePath()
			if err != nil {
				return err
			}

			if err := idtools.MkdirAllAndChown(shmPath, 0o700, rootIDs); err != nil {
				return err
			}

			shmproperty := "mode=1777,size=" + strconv.FormatInt(ctr.HostConfig.ShmSize, 10)
			if err := unix.Mount("shm", shmPath, "tmpfs", uintptr(unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV), label.FormatMountLabel(shmproperty, ctr.GetMountLabel())); err != nil {
				return fmt.Errorf("mounting shm tmpfs: %s", err)
			}
			if err := os.Chown(shmPath, rootIDs.UID, rootIDs.GID); err != nil {
				return err
			}
			ctr.ShmPath = shmPath
		}

	default:
		return fmt.Errorf("invalid IPC mode: %v", ipcMode)
	}

	return nil
}

func (daemon *Daemon) setupSecretDir(ctr *container.Container) (setupErr error) {
	if len(ctr.SecretReferences) == 0 && len(ctr.ConfigReferences) == 0 {
		return nil
	}

	if err := daemon.createSecretsDir(ctr); err != nil {
		return err
	}
	defer func() {
		if setupErr != nil {
			daemon.cleanupSecretDir(ctr)
		}
	}()

	if ctr.DependencyStore == nil {
		return fmt.Errorf("secret store is not initialized")
	}

	// retrieve possible remapped range start for root UID, GID
	rootIDs := daemon.idMapping.RootPair()

	for _, s := range ctr.SecretReferences {
		// TODO (ehazlett): use type switch when more are supported
		if s.File == nil {
			log.G(context.TODO()).Error("secret target type is not a file target")
			continue
		}

		// secrets are created in the SecretMountPath on the host, at a
		// single level
		fPath, err := ctr.SecretFilePath(*s)
		if err != nil {
			return errors.Wrap(err, "error getting secret file path")
		}
		if err := idtools.MkdirAllAndChown(filepath.Dir(fPath), 0o700, rootIDs); err != nil {
			return errors.Wrap(err, "error creating secret mount path")
		}

		log.G(context.TODO()).WithFields(log.Fields{
			"name": s.File.Name,
			"path": fPath,
		}).Debug("injecting secret")
		secret, err := ctr.DependencyStore.Secrets().Get(s.SecretID)
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

	for _, configRef := range ctr.ConfigReferences {
		// TODO (ehazlett): use type switch when more are supported
		if configRef.File == nil {
			// Runtime configs are not mounted into the container, but they're
			// a valid type of config so we should not error when we encounter
			// one.
			if configRef.Runtime == nil {
				log.G(context.TODO()).Error("config target type is not a file or runtime target")
			}
			// However, in any case, this isn't a file config, so we have no
			// further work to do
			continue
		}

		fPath, err := ctr.ConfigFilePath(*configRef)
		if err != nil {
			return errors.Wrap(err, "error getting config file path for container")
		}
		if err := idtools.MkdirAllAndChown(filepath.Dir(fPath), 0o700, rootIDs); err != nil {
			return errors.Wrap(err, "error creating config mount path")
		}

		log.G(context.TODO()).WithFields(log.Fields{
			"name": configRef.File.Name,
			"path": fPath,
		}).Debug("injecting config")
		config, err := ctr.DependencyStore.Configs().Get(configRef.ConfigID)
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

	return daemon.remountSecretDir(ctr)
}

// createSecretsDir is used to create a dir suitable for storing container secrets.
// In practice this is using a tmpfs mount and is used for both "configs" and "secrets"
func (daemon *Daemon) createSecretsDir(ctr *container.Container) error {
	// retrieve possible remapped range start for root UID, GID
	rootIDs := daemon.idMapping.RootPair()
	dir, err := ctr.SecretMountPath()
	if err != nil {
		return errors.Wrap(err, "error getting container secrets dir")
	}

	// create tmpfs
	if err := idtools.MkdirAllAndChown(dir, 0o700, rootIDs); err != nil {
		return errors.Wrap(err, "error creating secret local mount path")
	}

	tmpfsOwnership := fmt.Sprintf("uid=%d,gid=%d", rootIDs.UID, rootIDs.GID)
	if err := mount.Mount("tmpfs", dir, "tmpfs", "nodev,nosuid,noexec,"+tmpfsOwnership); err != nil {
		return errors.Wrap(err, "unable to setup secret mount")
	}
	return nil
}

func (daemon *Daemon) remountSecretDir(ctr *container.Container) error {
	dir, err := ctr.SecretMountPath()
	if err != nil {
		return errors.Wrap(err, "error getting container secrets path")
	}
	if err := label.Relabel(dir, ctr.MountLabel, false); err != nil {
		log.G(context.TODO()).WithError(err).WithField("dir", dir).Warn("Error while attempting to set selinux label")
	}
	rootIDs := daemon.idMapping.RootPair()
	tmpfsOwnership := fmt.Sprintf("uid=%d,gid=%d", rootIDs.UID, rootIDs.GID)

	// remount secrets ro
	if err := mount.Mount("tmpfs", dir, "tmpfs", "remount,ro,"+tmpfsOwnership); err != nil {
		return errors.Wrap(err, "unable to remount dir as readonly")
	}

	return nil
}

func (daemon *Daemon) cleanupSecretDir(ctr *container.Container) {
	dir, err := ctr.SecretMountPath()
	if err != nil {
		log.G(context.TODO()).WithError(err).WithField("container", ctr.ID).Warn("error getting secrets mount path for container")
	}
	if err := mount.RecursiveUnmount(dir); err != nil {
		log.G(context.TODO()).WithField("dir", dir).WithError(err).Warn("Error while attempting to unmount dir, this may prevent removal of container.")
	}
	if err := os.RemoveAll(dir); err != nil {
		log.G(context.TODO()).WithField("dir", dir).WithError(err).Error("Error removing dir.")
	}
}

func killProcessDirectly(ctr *container.Container) error {
	pid := ctr.GetPID()
	if pid == 0 {
		// Ensure that we don't kill ourselves
		return nil
	}

	if err := unix.Kill(pid, syscall.SIGKILL); err != nil {
		if err != unix.ESRCH {
			return errdefs.System(err)
		}
		err = errNoSuchProcess{pid, syscall.SIGKILL}
		log.G(context.TODO()).WithError(err).WithField("container", ctr.ID).Debug("no such process")
		return err
	}

	// In case there were some exceptions(e.g., state of zombie and D)
	if process.Alive(pid) {
		// Since we can not kill a zombie pid, add zombie check here
		isZombie, err := process.Zombie(pid)
		if err != nil {
			log.G(context.TODO()).WithError(err).WithField("container", ctr.ID).Warn("Container state is invalid")
			return err
		}
		if isZombie {
			return errdefs.System(errors.Errorf("container %s PID %d is zombie and can not be killed. Use the --init option when creating containers to run an init inside the container that forwards signals and reaps processes", stringid.TruncateID(ctr.ID), pid))
		}
	}
	return nil
}

func isLinkable(child *container.Container) bool {
	// A container is linkable only if it belongs to the default network
	_, ok := child.NetworkSettings.Networks[runconfig.DefaultDaemonNetworkMode().NetworkName()]
	return ok
}

// TODO(aker): remove when we make the default bridge network behave like any other network
func enableIPOnPredefinedNetwork() bool {
	return false
}

// serviceDiscoveryOnDefaultNetwork indicates if service discovery is supported on the default network
// TODO(aker): remove when we make the default bridge network behave like any other network
func serviceDiscoveryOnDefaultNetwork() bool {
	return false
}

func buildSandboxPlatformOptions(ctr *container.Container, cfg *config.Config, sboxOptions *[]libnetwork.SandboxOption) error {
	var err error

	// In host-mode networking, the container does not have its own networking
	// namespace, so `/etc/hosts` should be the same as on the host itself. Setting
	// OptionOriginHostsPath means the container will get a copy of '/etc/hosts' from
	// the host filesystem.
	// Note that containers with "container" networking have been handled in
	// "initializeNetworking()", so do not have to be accounted for here.
	if ctr.HostConfig.NetworkMode.IsHost() {
		*sboxOptions = append(
			*sboxOptions,
			libnetwork.OptionOriginHostsPath("/etc/hosts"),
		)
	}

	originResolvConfPath := "/etc/resolv.conf"
	// Allow tests to point at their own resolv.conf file.
	if envPath := os.Getenv("DOCKER_TEST_RESOLV_CONF_PATH"); envPath != "" {
		log.G(context.TODO()).Infof("Using OriginResolvConfPath from env: %s", envPath)
		originResolvConfPath = envPath
	}
	*sboxOptions = append(*sboxOptions, libnetwork.OptionOriginResolvConfPath(originResolvConfPath))

	ctr.HostsPath, err = ctr.GetRootResourcePath("hosts")
	if err != nil {
		return err
	}
	*sboxOptions = append(*sboxOptions, libnetwork.OptionHostsPath(ctr.HostsPath))

	ctr.ResolvConfPath, err = ctr.GetRootResourcePath("resolv.conf")
	if err != nil {
		return err
	}
	*sboxOptions = append(*sboxOptions, libnetwork.OptionResolvConfPath(ctr.ResolvConfPath))

	return nil
}

func (daemon *Daemon) initializeNetworkingPaths(ctr *container.Container, nc *container.Container) error {
	ctr.HostnamePath = nc.HostnamePath
	ctr.HostsPath = nc.HostsPath
	ctr.ResolvConfPath = nc.ResolvConfPath
	return nil
}

func (daemon *Daemon) setupContainerMountsRoot(ctr *container.Container) error {
	// get the root mount path so we can make it unbindable
	p, err := ctr.MountsResourcePath("")
	if err != nil {
		return err
	}
	return idtools.MkdirAllAndChown(p, 0o710, idtools.Identity{UID: idtools.CurrentIdentity().UID, GID: daemon.IdentityMapping().RootPair().GID})
}
