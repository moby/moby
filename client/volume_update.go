package client

import (
	"encoding/json"

	"github.com/docker/docker/api/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	"golang.org/x/net/context"
)

// VolumeUpdate updates a volume in the docker host.
func (cli *Client) VolumeUpdate(ctx context.Context, volumeID string, options volumetypes.VolumesUpdateBody) (types.Volume, error) {
	var volume types.Volume
	resp, err := cli.put(ctx, "/volumes/"+volumeID, nil, options, nil)
	if err != nil {
		return volume, err
	}
	err = json.NewDecoder(resp.body).Decode(&volume)
	ensureReaderClosed(resp)
	return volume, err
}
