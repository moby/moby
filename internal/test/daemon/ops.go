package daemon

import "github.com/docker/docker/internal/test/environment"

// WithExperimental sets the daemon in experimental mode
func WithExperimental(d *Daemon) {
	d.experimental = true
	d.init = true
}

// WithInit sets the daemon init
func WithInit(d *Daemon) {
	d.init = true
}

// WithDockerdBinary sets the dockerd binary to the specified one
func WithDockerdBinary(dockerdBinary string) func(*Daemon) {
	return func(d *Daemon) {
		d.dockerdBinary = dockerdBinary
	}
}

// WithSwarmPort sets the swarm port to use for swarm mode
func WithSwarmPort(port int) func(*Daemon) {
	return func(d *Daemon) {
		d.SwarmPort = port
	}
}

// WithSwarmListenAddr sets the swarm listen addr to use for swarm mode
func WithSwarmListenAddr(listenAddr string) func(*Daemon) {
	return func(d *Daemon) {
		d.swarmListenAddr = listenAddr
	}
}

// WithEnvironment sets options from internal/test/environment.Execution struct
func WithEnvironment(e environment.Execution) func(*Daemon) {
	return func(d *Daemon) {
		if e.DaemonInfo.ExperimentalBuild {
			d.experimental = true
		}
	}
}
