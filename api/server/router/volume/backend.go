package volume // import "github.com/docker/docker/api/server/router/volume"

import (
	"context"

	"github.com/docker/docker/volume/service/opts"
	// TODO return types need to be refactored into pkg
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// Backend is the methods that need to be implemented to provide
// volume specific functionality
type Backend interface {
	List(ctx context.Context, filter filters.Args) ([]*volume.Volume, []string, error)
	Get(ctx context.Context, name string, opts ...opts.GetOption) (*volume.Volume, error)
	Create(ctx context.Context, name, driverName string, opts ...opts.CreateOption) (*volume.Volume, error)
	Remove(ctx context.Context, name string, opts ...opts.RemoveOption) error
	Prune(ctx context.Context, pruneFilters filters.Args) (*types.VolumesPruneReport, error)
}
