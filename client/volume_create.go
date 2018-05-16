package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"

	volumetypes "github.com/docker/docker/api/types/volume"
)

// VolumeCreate creates a volume in the docker host.
func (cli *Client) VolumeCreate(ctx context.Context, options volumetypes.VolumeCreateBody) (volumetypes.Volume, error) {
	var volume volumetypes.Volume
	resp, err := cli.post(ctx, "/volumes/create", nil, options, nil)
	if err != nil {
		return volume, err
	}
	err = json.NewDecoder(resp.body).Decode(&volume)
	ensureReaderClosed(resp)
	return volume, err
}
