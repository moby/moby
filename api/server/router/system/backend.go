package system

import (
	"encoding/json"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"golang.org/x/net/context"
)

// Backend is the methods that need to be implemented to provide
// system specific functionality.
type Backend interface {
	SystemInfo() (*types.Info, error)
	SystemVersion() types.Version
	SystemDiskUsage(ctx context.Context) (*types.DiskUsage, error)
	AuthenticateToRegistry(ctx context.Context, authConfig *types.AuthConfig) (string, string, error)
	Events(ctx context.Context, since, until time.Time, enc *json.Encoder, filter filters.Args) error
}
