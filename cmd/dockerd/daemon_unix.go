//go:build !windows
// +build !windows

package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libcontainerd/supervisor"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/pkg/homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	desiredUmask := 0022
	unix.Umask(desiredUmask)
	if umask := unix.Umask(desiredUmask); umask != desiredUmask {
		return errors.Errorf("failed to set umask: expected %#o, got %#o", desiredUmask, umask)
	}

	return nil
}

func (cli *DaemonCli) getPlatformContainerdDaemonOpts() ([]supervisor.DaemonOpt, error) {
	opts := []supervisor.DaemonOpt{
		// TODO(thaJeztah) change this to use /proc/self/oom_score_adj instead,
		// which would allow us to set the correct score even if dockerd's score
		// was set through other means (such as systemd or "manually").
		supervisor.WithOOMScore(cli.Config.OOMScoreAdjust), //nolint:staticcheck // ignore SA1019 (WithOOMScore is deprecated); will be removed in the next release.
	}
	if cli.Config.OOMScoreAdjust != 0 {
		logrus.Warn(`DEPRECATED: The "oom-score-adjust" config parameter and the dockerd "--oom-score-adjust" option will be removed in the next release.`)
	}
	return opts, nil
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

	logrus.Info("containerd not running, starting managed containerd")
	opts, err := cli.getContainerdDaemonOpts()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate containerd options")
	}

	r, err := supervisor.Start(ctx, filepath.Join(cli.Root, "containerd"), filepath.Join(cli.ExecRoot, "containerd"), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start containerd")
	}
	cli.ContainerdAddr = r.Address()

	// Try to wait for containerd to shutdown
	return r.WaitTimeout, nil
}
