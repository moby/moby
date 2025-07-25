package system

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
)

// Backend is the methods that need to be implemented to provide
// system specific functionality.
type Backend interface {
	SystemInfo(context.Context) (*system.Info, error)
	SystemVersion(context.Context) (types.Version, error)
	SystemDiskUsage(ctx context.Context, opts backend.DiskUsageOptions) (*backend.DiskUsage, error)
	SubscribeToEvents(since, until time.Time, ef filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(chan interface{})
	AuthenticateToRegistry(ctx context.Context, authConfig *registry.AuthConfig) (string, string, error)
}

// ClusterBackend is all the methods that need to be implemented
// to provide cluster system specific functionality.
type ClusterBackend interface {
	Info(context.Context) swarm.Info
}

// BuildBackend provides build specific system information.
type BuildBackend interface {
	DiskUsage(context.Context) ([]*build.CacheRecord, error)
}

// StatusProvider provides methods to get the swarm status of the current node.
type StatusProvider interface {
	Status() string
}
