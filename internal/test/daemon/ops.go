package daemon

// WithExperimental sets the daemon in experimental mode
func WithExperimental(d *Daemon) {
	d.experimental = true
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
