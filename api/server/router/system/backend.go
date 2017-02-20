package system

import (
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/system"
	"golang.org/x/net/context"
)

// Backend is the methods that need to be implemented to provide
// system specific functionality.
type Backend interface {
	SystemInfo() (*types.Info, error)
	SystemVersion() system.VersionOKBody
	SystemDiskUsage() (*types.DiskUsage, error)
	SubscribeToEvents(since, until time.Time, ef filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(chan interface{})
	AuthenticateToRegistry(ctx context.Context, authConfig *types.AuthConfig) (string, string, error)
}
