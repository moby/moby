package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"
import (
	"path/filepath"

	"github.com/containerd/containerd/services/server/config"
)

const (
	grpcPipeName  = `\\.\pipe\containerd-containerd`
	debugPipeName = `\\.\pipe\containerd-debug`
)

// WithPlatformDefaults sets the default options for the platform.
func WithPlatformDefaults(rootDir string) DaemonOpt {
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
			r.GRPC.Address = grpcPipeName
		}
		if r.Debug.Address == "" {
			r.Debug.Address = debugPipeName
		}
		return nil
	}
}
