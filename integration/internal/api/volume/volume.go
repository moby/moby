package volume

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// Create creates a volume using the named driver with the specified options
func Create(client client.APIClient, driver string, opts map[string]string) (string, error) {
	volReq := volumetypes.VolumesCreateBody{
		Driver:     driver,
		DriverOpts: opts,
		Name:       "",
	}

	ctx := context.Background()
	vol, err := client.VolumeCreate(ctx, volReq)
	if err != nil {
		return "", err
	}
	return vol.Name, nil
}

// Rm removes the volume named
func Rm(client client.APIClient, name string) error {
	ctx := context.Background()
	return client.VolumeRemove(ctx, name, false)
}

// Ls lists the volumes available
func Ls(client client.APIClient) ([]string, error) {
	ctx := context.Background()
	volumes, err := client.VolumeList(ctx, filters.Args{})
	if err != nil {
		return []string{}, err
	}

	names := []string{}
	for _, volume := range volumes.Volumes {
		names = append(names, volume.Name)
	}

	return names, nil
}

// Prune removes all volumes not used by at least one container
func Prune(client client.APIClient) (types.VolumesPruneReport, error) {
	ctx := context.Background()

	return client.VolumesPrune(ctx, filters.Args{})
}

// Inspect retrieves detailed information about the named volume
func Inspect(client client.APIClient, name string) (types.Volume, error) {
	ctx := context.Background()
	return client.VolumeInspect(ctx, name)
}
