package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

// WithLogLevel defines which log level to starts containerd with.
// This only makes sense if WithStartDaemon() was set to true.
func WithLogLevel(lvl string) DaemonOpt {
	return func(r *remote) error {
		r.Debug.Level = lvl
		return nil
	}
}

// WithPlugin allow configuring a containerd plugin
// configuration values passed needs to be quoted if quotes are needed in
// the toml format.
func WithPlugin(name string, conf interface{}) DaemonOpt {
	return func(r *remote) error {
		r.Plugins[name] = conf
		return nil
	}
}
