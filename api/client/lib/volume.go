package lib

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

// VolumeList returns the volumes configured in the docker host.
func (cli *Client) VolumeList(filter filters.Args) (types.VolumesListResponse, error) {
	var volumes types.VolumesListResponse
	query := url.Values{}

	if filter.Len() > 0 {
		filterJSON, err := filters.ToParam(filter)
		if err != nil {
			return volumes, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get("/volumes", query, nil)
	if err != nil {
		return volumes, err
	}
	defer ensureReaderClosed(resp)

	err = json.NewDecoder(resp.body).Decode(&volumes)
	return volumes, err
}

// VolumeInspect returns the information about a specific volume in the docker host.
func (cli *Client) VolumeInspect(volumeID string) (types.Volume, error) {
	var volume types.Volume
	resp, err := cli.get("/volumes/"+volumeID, nil, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return volume, volumeNotFoundError{volumeID}
		}
		return volume, err
	}
	defer ensureReaderClosed(resp)
	err = json.NewDecoder(resp.body).Decode(&volume)
	return volume, err
}

// VolumeCreate creates a volume in the docker host.
func (cli *Client) VolumeCreate(options types.VolumeCreateRequest) (types.Volume, error) {
	var volume types.Volume
	resp, err := cli.post("/volumes/create", nil, options, nil)
	if err != nil {
		return volume, err
	}
	defer ensureReaderClosed(resp)
	err = json.NewDecoder(resp.body).Decode(&volume)
	return volume, err
}

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(volumeID string) error {
	resp, err := cli.delete("/volumes/"+volumeID, nil, nil)
	ensureReaderClosed(resp)
	return err
}
