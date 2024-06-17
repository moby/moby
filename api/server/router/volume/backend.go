package volume // import "github.com/docker/docker/api/server/router/volume"

import (
	"context"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/volume/service/opts"
)

// Backend is the methods that need to be implemented to provide
// volume specific functionality
type Backend interface {
	List(ctx context.Context, filter filters.Args) ([]*volume.Volume, []string, error)
	Get(ctx context.Context, name string, opts ...opts.GetOption) (*volume.Volume, error)
	Create(ctx context.Context, name, driverName string, opts ...opts.CreateOption) (*volume.Volume, error)
	Remove(ctx context.Context, name string, opts ...opts.RemoveOption) error
	Prune(ctx context.Context, pruneFilters filters.Args) (*volume.PruneReport, error)
}

// ClusterBackend is the backend used for Swarm Cluster Volumes. Regular
// volumes go through the volume service, but to avoid across-dependency
// between the cluster package and the volume package, we simply provide two
// backends here.
type ClusterBackend interface {
	GetVolume(nameOrID string) (volume.Volume, error)
	GetVolumes(options volume.ListOptions) ([]*volume.Volume, error)
	CreateVolume(volume volume.CreateOptions) (*volume.Volume, error)
	RemoveVolume(nameOrID string, force bool) error
	UpdateVolume(nameOrID string, version uint64, volume volume.UpdateOptions) error
	IsManager() bool
}
