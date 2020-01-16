package system // import "github.com/moby/moby/api/server/router/system"

import (
	"context"
	"time"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/swarm"
)

// Backend is the methods that need to be implemented to provide
// system specific functionality.
type Backend interface {
	SystemInfo() (*types.Info, error)
	SystemVersion() types.Version
	SystemDiskUsage(ctx context.Context) (*types.DiskUsage, error)
	SubscribeToEvents(since, until time.Time, ef filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(chan interface{})
	AuthenticateToRegistry(ctx context.Context, authConfig *types.AuthConfig) (string, string, error)
}

// ClusterBackend is all the methods that need to be implemented
// to provide cluster system specific functionality.
type ClusterBackend interface {
	Info() swarm.Info
}
