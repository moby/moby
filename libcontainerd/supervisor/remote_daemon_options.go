package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

// WithLogLevel defines which log level to start containerd with.
func WithLogLevel(lvl string) DaemonOpt {
	return func(r *remote) error {
		if lvl == "info" {
			// both dockerd and containerd default log-level is "info",
			// so don't pass the default.
			lvl = ""
		}
		r.logLevel = lvl
		return nil
	}
}

// WithLogFormat defines the containerd log format.
// This only makes sense if WithStartDaemon() was set to true.
func WithLogFormat(format string) DaemonOpt {
	return func(r *remote) error {
		r.Debug.Format = format
		return nil
	}
}

// WithCRIDisabled disables the CRI plugin.
func WithCRIDisabled() DaemonOpt {
	return func(r *remote) error {
		r.DisabledPlugins = append(r.DisabledPlugins, "io.containerd.grpc.v1.cri")
		return nil
	}
}
