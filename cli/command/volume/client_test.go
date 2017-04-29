package volume

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type fakeClient struct {
	client.Client
	volumeCreateFunc  func(volumetypes.VolumesCreateBody) (types.Volume, error)
	volumeInspectFunc func(volumeID string) (types.Volume, error)
	volumeListFunc    func(filter filters.Args) (volumetypes.VolumesListOKBody, error)
	volumeRemoveFunc  func(volumeID string, force bool) error
	volumePruneFunc   func(filter filters.Args) (types.VolumesPruneReport, error)
}

func (c *fakeClient) VolumeCreate(ctx context.Context, options volumetypes.VolumesCreateBody) (types.Volume, error) {
	if c.volumeCreateFunc != nil {
		return c.volumeCreateFunc(options)
	}
	return types.Volume{}, nil
}

func (c *fakeClient) VolumeInspect(ctx context.Context, volumeID string) (types.Volume, error) {
	if c.volumeInspectFunc != nil {
		return c.volumeInspectFunc(volumeID)
	}
	return types.Volume{}, nil
}

func (c *fakeClient) VolumeList(ctx context.Context, filter filters.Args) (volumetypes.VolumesListOKBody, error) {
	if c.volumeListFunc != nil {
		return c.volumeListFunc(filter)
	}
	return volumetypes.VolumesListOKBody{}, nil
}

func (c *fakeClient) VolumesPrune(ctx context.Context, filter filters.Args) (types.VolumesPruneReport, error) {
	if c.volumePruneFunc != nil {
		return c.volumePruneFunc(filter)
	}
	return types.VolumesPruneReport{}, nil
}

func (c *fakeClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	if c.volumeRemoveFunc != nil {
		return c.volumeRemoveFunc(volumeID, force)
	}
	return nil
}
