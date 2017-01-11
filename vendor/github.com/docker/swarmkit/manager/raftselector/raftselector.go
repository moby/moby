package raftselector

import (
	"errors"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
)

// ConnProvider is basic interface for connecting API package(raft proxy in particular)
// to manager/state/raft package without import cycles. It provides only one
// method for obtaining connection to leader.
type ConnProvider interface {
	LeaderConn(ctx context.Context) (*grpc.ClientConn, error)
}

// ErrIsLeader is returned from LeaderConn method when current machine is leader.
// It's just shim between packages to avoid import cycles.
var ErrIsLeader = errors.New("current node is leader")
