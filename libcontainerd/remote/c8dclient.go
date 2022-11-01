package remote // import "github.com/docker/docker/libcontainerd/remote"

import (
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/pkg/dialer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// NewContainerdClient returns a new containerd client using reasonable default
// gRPC dial and connection timeout options.
func NewContainerdClient(address string, opts ...containerd.ClientOpt) (*containerd.Client, error) {
	backoffConfig := backoff.DefaultConfig
	backoffConfig.MaxDelay = 3 * time.Second
	connParams := grpc.ConnectParams{
		Backoff: backoffConfig,
	}
	gopts := []grpc.DialOption{
		// WithBlock makes sure that the following containerd request
		// is reliable.
		//
		// NOTE: In one edge case with high load pressure, kernel kills
		// dockerd, containerd and containerd-shims caused by OOM.
		// When both dockerd and containerd restart, but containerd
		// will take time to recover all the existing containers. Before
		// containerd serving, dockerd will failed with gRPC error.
		// That bad thing is that restore action will still ignore the
		// any non-NotFound errors and returns running state for
		// already stopped container. It is unexpected behavior. And
		// we need to restart dockerd to make sure that anything is OK.
		//
		// It is painful. Add WithBlock can prevent the edge case. And
		// n common case, the containerd will be serving in shortly.
		// It is not harm to add WithBlock for containerd connection.
		grpc.WithBlock(),

		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(connParams),
		grpc.WithContextDialer(dialer.ContextDialer),

		// TODO(stevvooe): We may need to allow configuration of this on the client.
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
	}

	opts = append([]containerd.ClientOpt{
		containerd.WithDialOpts(gopts),
		containerd.WithTimeout(60 * time.Second),
	}, opts...)
	return containerd.New(address, opts...)
}
