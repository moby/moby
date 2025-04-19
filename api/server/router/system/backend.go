package system // import "github.com/docker/docker/api/server/router/system"

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
)

// DiskUsageOptions holds parameters for system disk usage query.
type DiskUsageOptions struct {
	// Containers controls whether container disk usage should be computed.
	Containers bool

	// Images controls whether image disk usage should be computed.
	Images bool

	// Volumes controls whether volume disk usage should be computed.
	Volumes bool
}

// Backend is the methods that need to be implemented to provide
// system specific functionality.
type Backend interface {
	SystemInfo(context.Context) (*system.Info, error)
	SystemVersion(context.Context) (types.Version, error)
	SystemDiskUsage(ctx context.Context, opts DiskUsageOptions) (*system.DiskUsage, error)
	SubscribeToEvents(since, until time.Time, ef filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(chan interface{})
	AuthenticateToRegistry(ctx context.Context, authConfig *registry.AuthConfig) (string, string, error)
}

// ClusterBackend is all the methods that need to be implemented
// to provide cluster system specific functionality.
type ClusterBackend interface {
	Info(context.Context) swarm.Info
}

// StatusProvider provides methods to get the swarm status of the current node.
type StatusProvider interface {
	Status() string
}
