package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

const (
	grpcPipeName  = `\\.\pipe\containerd-containerd`
	debugPipeName = `\\.\pipe\containerd-debug`
)

// WithPlatformDefaults sets the default options for the platform.
func WithPlatformDefaults() DaemonOpt {
	return func(r *remote) error {
		if r.GRPC.Address == "" {
			r.GRPC.Address = grpcPipeName
		}
		if r.Debug.Address == "" {
			r.Debug.Address = debugPipeName
		}
		return nil
	}
}
