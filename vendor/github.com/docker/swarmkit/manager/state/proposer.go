package state

import (
	"github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

// A Proposer can propose actions to a cluster.
type Proposer interface {
	// ProposeValue adds storeAction to the distributed log. If this
	// completes successfully, ProposeValue calls cb to commit the
	// proposed changes. The callback is necessary for the Proposer to make
	// sure that the changes are committed before it interacts further
	// with the store.
	ProposeValue(ctx context.Context, storeAction []*api.StoreAction, cb func()) error
	GetVersion() *api.Version
}
