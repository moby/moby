package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

// WithRemoteAddr sets the external containerd socket to connect to.
func WithRemoteAddr(addr string) DaemonOpt {
	return func(r *remote) error {
		r.GRPC.Address = addr
		return nil
	}
}

// WithRemoteAddrUser sets the uid and gid to create the RPC address with
func WithRemoteAddrUser(uid, gid int) DaemonOpt {
	return func(r *remote) error {
		r.GRPC.UID = uid
		r.GRPC.GID = gid
		return nil
	}
}

// WithLogLevel defines which log level to starts containerd with.
// This only makes sense if WithStartDaemon() was set to true.
func WithLogLevel(lvl string) DaemonOpt {
	return func(r *remote) error {
		r.Debug.Level = lvl
		return nil
	}
}

// WithDebugAddress defines at which location the debug GRPC connection
// should be made
func WithDebugAddress(addr string) DaemonOpt {
	return func(r *remote) error {
		r.Debug.Address = addr
		return nil
	}
}

// WithMetricsAddress defines at which location the debug GRPC connection
// should be made
func WithMetricsAddress(addr string) DaemonOpt {
	return func(r *remote) error {
		r.Metrics.Address = addr
		return nil
	}
}

// WithPlugin allow configuring a containerd plugin
// configuration values passed needs to be quoted if quotes are needed in
// the toml format.
func WithPlugin(name string, conf interface{}) DaemonOpt {
	return func(r *remote) error {
		r.pluginConfs.Plugins[name] = conf
		return nil
	}
}
