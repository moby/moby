package client

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	"golang.org/x/net/context"
)

// VolumeCreate creates a volume in the docker host.
func (cli *Client) VolumeCreate(ctx context.Context, options volumetypes.VolumesCreateBody) (types.Volume, error) {
	var volume types.Volume
	resp, err := cli.post(ctx, "/volumes/create", nil, options, nil)
	if resp.statusCode == http.StatusNotModified {
		// StatusNotModified does not carry a body. We assign the name
		// to the returned types.Volume so that it could be outputed correctly.
		volume.Name = options.Name
		ensureReaderClosed(resp)
		return volume, nil
	}
	if err != nil {
		return volume, err
	}
	err = json.NewDecoder(resp.body).Decode(&volume)
	ensureReaderClosed(resp)
	return volume, err
}
