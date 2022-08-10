package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"
import (
	"path/filepath"

	"github.com/containerd/containerd/defaults"
)

const (
	sockFile      = "containerd.sock"
	debugSockFile = "containerd-debug.sock"
)

// withPlatformDefaults sets the default options for the platform.
func withPlatformDefaults() DaemonOpt {
	return func(r *remote) error {
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
