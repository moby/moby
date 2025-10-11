package daemon

import (
	"net/netip"
	"os"
	"os/user"

	"github.com/moby/moby/v2/internal/testutil/environment"
)

// Option is used to configure a daemon.
type Option func(*Daemon)

// WithContainerdSocket sets the --containerd option on the daemon.
// Use an empty string to remove the option.
//
// If unset the --containerd option will be used with a default value.
func WithContainerdSocket(socket string) Option {
	return func(d *Daemon) {
		d.containerdSocket = socket
	}
}

func WithUserNsRemap(remap string) Option {
	return func(d *Daemon) {
		d.usernsRemap = remap
		// The dind container used by the CI has the env var DOCKER_GRAPHDRIVER
		// set to 'overlayfs' which is a valid driver when the containerd image
		// store is used, but not a valid graphdriver. OTOH the test daemon
		// started by this package uses DOCKER_GRAPHDRIVER to set the storage
		// backend. However, the daemon doesn't enable the containerd image
		// store automatically when userns remapping is enabled, so using the
		// storage driver set through DOCKER_GRAPHDRIVER will cause the daemon
		// to fail to start. This should be removed once a proper fix for [1]
		// is implemented.
		//
		// [1]: https://github.com/moby/moby/issues/47377
		d.storageDriver = ""
		if storage := os.Getenv("DOCKER_GRAPHDRIVER"); storage == "overlayfs" {
			d.storageDriver = "overlay2"
		}
	}
}

// WithDefaultCgroupNamespaceMode sets the default cgroup namespace mode for the daemon
func WithDefaultCgroupNamespaceMode(mode string) Option {
	return func(d *Daemon) {
		d.defaultCgroupNamespaceMode = mode
	}
}

// WithTestLogger causes the daemon to log certain actions to the provided test.
func WithTestLogger(t LogT) Option {
	return func(d *Daemon) {
		d.log = t
	}
}

// WithExperimental sets the daemon in experimental mode
func WithExperimental() Option {
	return func(d *Daemon) {
		d.experimental = true
	}
}

// WithInit sets the daemon init
func WithInit() Option {
	return func(d *Daemon) {
		d.init = true
	}
}

// WithDockerdBinary sets the dockerd binary to the specified one
func WithDockerdBinary(dockerdBinary string) Option {
	return func(d *Daemon) {
		d.dockerdBinary = dockerdBinary
	}
}

// WithSwarmPort sets the swarm port to use for swarm mode
func WithSwarmPort(port int) Option {
	return func(d *Daemon) {
		d.SwarmPort = port
	}
}

// WithSwarmListenAddr sets the swarm listen addr to use for swarm mode
func WithSwarmListenAddr(listenAddr string) Option {
	return func(d *Daemon) {
		d.swarmListenAddr = listenAddr
	}
}

// WithSwarmIptables enabled/disables iptables for swarm nodes
func WithSwarmIptables(useIptables bool) Option {
	return func(d *Daemon) {
		d.swarmWithIptables = useIptables
	}
}

// WithSwarmDefaultAddrPool sets the swarm default address pool to use for swarm mode
func WithSwarmDefaultAddrPool(defaultAddrPool ...netip.Prefix) Option {
	return func(d *Daemon) {
		d.DefaultAddrPool = defaultAddrPool
	}
}

// WithSwarmDefaultAddrPoolSubnetSize sets the subnet length mask of swarm default address pool to use for swarm mode
func WithSwarmDefaultAddrPoolSubnetSize(subnetSize uint32) Option {
	return func(d *Daemon) {
		d.SubnetSize = subnetSize
	}
}

// WithSwarmDataPathPort sets the  swarm datapath port to use for swarm mode
func WithSwarmDataPathPort(datapathPort uint32) Option {
	return func(d *Daemon) {
		d.DataPathPort = datapathPort
	}
}

// WithEnvironment sets options from testutil/environment.Execution struct
func WithEnvironment(e environment.Execution) Option {
	return func(d *Daemon) {
		if e.DaemonInfo.ExperimentalBuild {
			d.experimental = true
		}
	}
}

// WithStorageDriver sets store driver option
func WithStorageDriver(driver string) Option {
	return func(d *Daemon) {
		d.storageDriver = driver
	}
}

// WithRootlessUser sets the daemon to be rootless
func WithRootlessUser(username string) Option {
	return func(d *Daemon) {
		u, err := user.Lookup(username)
		if err != nil {
			panic(err)
		}
		d.rootlessUser = u
	}
}

// WithOOMScoreAdjust sets OOM score for the daemon
func WithOOMScoreAdjust(score int) Option {
	return func(d *Daemon) {
		d.OOMScoreAdjust = score
	}
}

// WithEnvVars sets additional environment variables for the daemon
func WithEnvVars(vars ...string) Option {
	return func(d *Daemon) {
		d.extraEnv = append(d.extraEnv, vars...)
	}
}

// WithResolvConf allows a test to provide content for a resolv.conf file to be used
// as the basis for resolv.conf in the container, instead of the host's /etc/resolv.conf.
func WithResolvConf(content string) Option {
	return func(d *Daemon) {
		d.resolvConfContent = content
	}
}
