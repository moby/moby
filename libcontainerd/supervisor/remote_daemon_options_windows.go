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
