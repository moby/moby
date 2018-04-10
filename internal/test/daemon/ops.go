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
