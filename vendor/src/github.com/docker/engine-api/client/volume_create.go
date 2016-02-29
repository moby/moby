package client

import (
	"encoding/json"

	"github.com/docker/engine-api/types"
)

// VolumeCreate creates a volume in the docker host.
func (cli *Client) VolumeCreate(options types.VolumeCreateRequest) (types.Volume, error) {
	var volume types.Volume
	resp, err := cli.post("/volumes/create", nil, options, nil)
	if err != nil {
		return volume, err
	}
	err = json.NewDecoder(resp.body).Decode(&volume)
	ensureReaderClosed(resp)
	return volume, err
}
