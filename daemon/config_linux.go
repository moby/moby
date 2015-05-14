package daemon

import (
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/ulimit"
)

var (
	defaultPidFile = "/var/run/docker.pid"
	defaultGraph   = "/var/lib/docker"
)

// Config defines the configuration of a docker daemon.
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker -d -e lxc`
type Config struct {
	CommonConfig

	// Fields below here are platform specific.
	EnableSelinuxSupport bool
	ExecOptions          []string
	GraphOptions         []string
	SocketGroup          string
	Ulimits              map[string]*ulimit.Ulimit
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallFlags() {
	// First handle install flags which are consistent cross-platform
	config.InstallCommonFlags()

	// Then platform-specific install flags
	opts.ListVar(&config.GraphOptions, []string{"-storage-opt"}, "Set storage driver options")
	opts.ListVar(&config.ExecOptions, []string{"-exec-opt"}, "Set exec driver options")
	flag.BoolVar(&config.EnableSelinuxSupport, []string{"-selinux-enabled"}, false, "Enable selinux support")
	flag.StringVar(&config.SocketGroup, []string{"G", "-group"}, "docker", "Group for the unix socket")
	config.Ulimits = make(map[string]*ulimit.Ulimit)
	opts.UlimitMapVar(config.Ulimits, []string{"-default-ulimit"}, "Set default ulimits for containers")
}
