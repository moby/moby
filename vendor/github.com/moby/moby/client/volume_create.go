package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/volume"
)

// VolumeCreateOptions specifies the options to create a volume.
type VolumeCreateOptions struct {
	Name              string
	Driver            string
	DriverOpts        map[string]string
	Labels            map[string]string
	ClusterVolumeSpec *volume.ClusterVolumeSpec
}

// VolumeCreateResult is the result of a volume creation.
type VolumeCreateResult struct {
	Volume volume.Volume
}

// VolumeCreate creates a volume in the docker host.
func (cli *Client) VolumeCreate(ctx context.Context, options VolumeCreateOptions) (VolumeCreateResult, error) {
	createRequest := volume.CreateRequest{
		Name:              options.Name,
		Driver:            options.Driver,
		DriverOpts:        options.DriverOpts,
		Labels:            options.Labels,
		ClusterVolumeSpec: options.ClusterVolumeSpec,
	}
	resp, err := cli.post(ctx, "/volumes/create", nil, createRequest, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumeCreateResult{}, err
	}

	var v volume.Volume
	err = json.NewDecoder(resp.Body).Decode(&v)
	return VolumeCreateResult{Volume: v}, err
}
