package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"
import (
	"path/filepath"

	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/services/server/config"
)

const (
	sockFile      = "containerd.sock"
	debugSockFile = "containerd-debug.sock"
)

// withPlatformDefaults sets the default options for the platform.
func withPlatformDefaults(rootDir string) DaemonOpt {
	return func(r *remote) error {
		if r.managedConfig {
			// custom configuration file is in use.
			return nil
		}
		r.managedConfig = true
		r.configFile = filepath.Join(r.stateDir, ConfigFile)
		r.Config = config.Config{
			Version: 2,
			Root:    filepath.Join(rootDir, "daemon"),
			State:   filepath.Join(r.stateDir, "daemon"),
		}
		if r.GRPC.Address == "" {
			r.GRPC.Address = filepath.Join(r.stateDir, sockFile)
		}
		if r.GRPC.MaxRecvMsgSize == 0 {
			r.GRPC.MaxRecvMsgSize = defaults.DefaultMaxRecvMsgSize
		}
		if r.GRPC.MaxSendMsgSize == 0 {
			r.GRPC.MaxSendMsgSize = defaults.DefaultMaxSendMsgSize
		}
		if r.Debug.Address == "" {
			r.Debug.Address = filepath.Join(r.stateDir, debugSockFile)
		}
		return nil
	}
}
