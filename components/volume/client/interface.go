package client

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/components/volume/types"
)

// VolumeAPIClient defines API client methods for the volumes
type VolumeAPIClient interface {
	VolumeCreate(ctx context.Context, options types.VolumeCreateRequest) (types.Volume, error)
	VolumeInspect(ctx context.Context, volumeID string) (types.Volume, error)
	VolumeInspectWithRaw(ctx context.Context, volumeID string) (types.Volume, []byte, error)
	VolumeList(ctx context.Context, filter filters.Args) (types.VolumesListResponse, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
}
