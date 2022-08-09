package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

// WithLogLevel defines which log level to starts containerd with.
// This only makes sense if WithStartDaemon() was set to true.
func WithLogLevel(lvl string) DaemonOpt {
	return func(r *remote) error {
		r.Debug.Level = lvl
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
