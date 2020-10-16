package daemon

import (
	"os/user"

	"github.com/docker/docker/testutil/environment"
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

// WithSwarmDefaultAddrPool sets the swarm default address pool to use for swarm mode
func WithSwarmDefaultAddrPool(defaultAddrPool []string) Option {
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
