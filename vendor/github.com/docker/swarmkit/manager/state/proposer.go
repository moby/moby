package state

import (
	"github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

// A Change includes a version number and a set of store actions from a
// particular log entry.
type Change struct {
	StoreActions []api.StoreAction
	Version      api.Version
}

// A Proposer can propose actions to a cluster.
type Proposer interface {
	// ProposeValue adds storeAction to the distributed log. If this
	// completes successfully, ProposeValue calls cb to commit the
	// proposed changes. The callback is necessary for the Proposer to make
	// sure that the changes are committed before it interacts further
	// with the store.
	ProposeValue(ctx context.Context, storeAction []api.StoreAction, cb func()) error
	// GetVersion returns the monotonic index of the most recent item in
	// the distributed log.
	GetVersion() *api.Version
	// ChangesBetween returns the changes starting after "from", up to and
	// including "to". If these changes are not available because the log
	// has been compacted, an error will be returned.
	ChangesBetween(from, to api.Version) ([]Change, error)
}
