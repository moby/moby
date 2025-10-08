package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/volume"
)

// VolumeList returns the volumes configured in the docker host.
func (cli *Client) VolumeList(ctx context.Context, options VolumeListOptions) (volume.ListResponse, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)
	resp, err := cli.get(ctx, "/volumes", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return volume.ListResponse{}, err
	}

	var volumes volume.ListResponse
	err = json.NewDecoder(resp.Body).Decode(&volumes)
	return volumes, err
}
