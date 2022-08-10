//go:build !windows

package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libcontainerd/supervisor"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/pkg/homedir"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func getDefaultDaemonConfigDir() (string, error) {
	if !honorXDG {
		return "/etc/docker", nil
	}
	// NOTE: CLI uses ~/.docker while the daemon uses ~/.config/docker, because
	// ~/.docker was not designed to store daemon configurations.
	// In future, the daemon directory may be renamed to ~/.config/moby-engine (?).
	configHome, err := homedir.GetConfigHome()
	if err != nil {
		return "", nil
	}
	return filepath.Join(configHome, "docker"), nil
}

func getDefaultDaemonConfigFile() (string, error) {
	dir, err := getDefaultDaemonConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.json"), nil
}

// setDefaultUmask sets the umask to 0022 to avoid problems
// caused by custom umask
func setDefaultUmask() error {
	desiredUmask := 0o022
	unix.Umask(desiredUmask)
	if umask := unix.Umask(desiredUmask); umask != desiredUmask {
		return errors.Errorf("failed to set umask: expected %#o, got %#o", desiredUmask, umask)
	}

	return nil
}

// setupConfigReloadTrap configures the SIGHUP signal to reload the configuration.
func (cli *DaemonCli) setupConfigReloadTrap() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, unix.SIGHUP)
	go func() {
		for range c {
			cli.reloadConfig()
		}
	}()
}

// getSwarmRunRoot gets the root directory for swarm to store runtime state
// For example, the control socket
func (cli *DaemonCli) getSwarmRunRoot() string {
	return filepath.Join(cli.Config.ExecRoot, "swarm")
}

// allocateDaemonPort ensures that there are no containers
// that try to use any port allocated for the docker server.
func allocateDaemonPort(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.Wrap(err, "error parsing tcp address")
	}

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return errors.Wrap(err, "error parsing tcp address")
	}

	var hostIPs []net.IP
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		hostIPs = append(hostIPs, parsedIP)
	} else if hostIPs, err = net.LookupIP(host); err != nil {
		return errors.Errorf("failed to lookup %s address in host specification", host)
	}

	pa := portallocator.Get()
	for _, hostIP := range hostIPs {
		if _, err := pa.RequestPort(hostIP, "tcp", intPort); err != nil {
			return errors.Errorf("failed to allocate daemon listening port %d (err: %v)", intPort, err)
		}
	}
	return nil
}

func newCgroupParent(config *config.Config) string {
	cgroupParent := "docker"
	useSystemd := daemon.UsingSystemd(config)
	if useSystemd {
		cgroupParent = "system.slice"
	}
	if config.CgroupParent != "" {
		cgroupParent = config.CgroupParent
	}
	if useSystemd {
		cgroupParent = cgroupParent + ":" + "docker" + ":"
	}
	return cgroupParent
}

func (cli *DaemonCli) initContainerd(ctx context.Context) (func(time.Duration) error, error) {
	if cli.ContainerdAddr != "" {
		// use system containerd at the given address.
		return nil, nil
	}

	systemContainerdAddr, ok, err := systemContainerdRunning(honorXDG)
	if err != nil {
		return nil, errors.Wrap(err, "could not determine whether the system containerd is running")
	}
	if ok {
		// detected a system containerd at the given address.
		cli.ContainerdAddr = systemContainerdAddr
		return nil, nil
	}

	log.G(ctx).Info("containerd not running, starting managed containerd")
	var opts []supervisor.DaemonOpt

	configFile, ok, err := getCustomConfigFile()
	if err != nil {
		return nil, err
	}
	if ok {
		// detected a custom containerd.toml config file with an address.
		log.G(ctx).Infof("using pre-existing containerd configuration file: %s", configFile)

		opts = append(opts,
			// We currently hard-code this to be relative to dockerd's exec-root
			// (e.g., /var/run/docker/containerd.pid on linux). Unlike the managed
			// config below, we don't use the "containerd" subdirectory so that
			// we don't have to create this subdirectory.
			supervisor.WithPIDFile(filepath.Join(cli.ExecRoot, supervisor.PIDFile)),
			supervisor.WithCustomConfigFile(configFile),
			supervisor.WithLogLevel(cli.LogLevel),
		)
		if cli.Debug {
			opts = append(opts, supervisor.WithLogLevel("debug"))
		}
	} else {
		// no existing configuration file found; set our own configuration.
		// this configuration will be written to a temporary containerd.toml
		// when the containerd instance is started.
		opts, err = cli.getContainerdDaemonOpts()
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate containerd options")
		}
	}

	r, err := supervisor.Start(ctx, filepath.Join(cli.ExecRoot, "containerd"), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start containerd")
	}
	cli.ContainerdAddr = r.Address()

	// Try to wait for containerd to shutdown
	return r.WaitTimeout, nil
}

// getCustomConfigFile checks if a custom containerd.toml configuration file is
// present in the daemon's default config directory. If found, it checks if the
// file is "valid" (contains an address for the containerd GRPC socket), and
// otherwise returns an error.
//
// Note that the daemon allows setting a custom location for the config-file,
// but not the config-directory. This means that with a custom "config-file",
// the containerd.toml and daemon.json may be in different locations. We should
// consider to either follow the same directory automatically, or to add a
// "--config-dir" option on the daemon, possibly deprecating the "--config-file"
// option.
func getCustomConfigFile() (fileName string, found bool, err error) {
	// TODO(thaJeztah) getDefaultDaemonConfigDir is not implemented / does not return a path on Windows
	// TODO(thaJeztah) consider making the daemon config-directory configurable.
	configDir, err := getDefaultDaemonConfigDir()
	if err != nil {
		return "", false, err
	}
	configFile := filepath.Join(configDir, supervisor.ConfigFile)
	_, err = supervisor.LoadConfigFile(configFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	return configFile, err == nil, nil
}
