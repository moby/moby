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

// WithCRIDisabled disables the CRI plugin.
func WithCRIDisabled() DaemonOpt {
	return func(r *remote) error {
		r.DisabledPlugins = append(r.DisabledPlugins, "io.containerd.grpc.v1.cri")
		return nil
	}
}

// WithPIDFile overrides the default location of the PID-file that's used by
// the supervisor.
func WithPIDFile(fileName string) DaemonOpt {
	return func(r *remote) error {
		r.pidFile = fileName
		return nil
	}
}

// WithCustomConfigFile configures the supervisor to use a custom containerd.toml
// configuration file instead of producing a generated config.
func WithCustomConfigFile(fileName string) DaemonOpt {
	return func(r *remote) error {
		conf, err := LoadConfigFile(fileName)
		if err != nil {
			return err
		}
		r.Config = *conf
		r.configFile = fileName
		r.managedConfig = false
		return nil
	}
}
