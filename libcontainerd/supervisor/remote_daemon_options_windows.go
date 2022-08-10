package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import "github.com/containerd/containerd/defaults"

const (
	grpcPipeName  = `\\.\pipe\containerd-containerd`
	debugPipeName = `\\.\pipe\containerd-debug`
)

// withPlatformDefaults sets the default options for the platform.
func withPlatformDefaults() DaemonOpt {
	return func(r *remote) error {
		if r.GRPC.Address == "" {
			r.GRPC.Address = grpcPipeName
		}
		if r.GRPC.MaxRecvMsgSize == 0 {
			r.GRPC.MaxRecvMsgSize = defaults.DefaultMaxRecvMsgSize
		}
		if r.GRPC.MaxSendMsgSize == 0 {
			r.GRPC.MaxSendMsgSize = defaults.DefaultMaxSendMsgSize
		}
		if r.Debug.Address == "" {
			r.Debug.Address = debugPipeName
		}
		return nil
	}
}
