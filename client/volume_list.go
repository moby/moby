package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/versions"
	"github.com/moby/moby/api/types/volume"
)

// VolumeList returns the volumes configured in the docker host.
func (cli *Client) VolumeList(ctx context.Context, options VolumeListOptions) (volume.ListResponse, error) {
	query := url.Values{}

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return volume.ListResponse{}, err
		}
		if cli.version != "" && versions.LessThan(cli.version, "1.22") {
			legacyFormat, err := encodeLegacyFilters(filterJSON)
			if err != nil {
				return volume.ListResponse{}, err
			}
			filterJSON = legacyFormat
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get(ctx, "/volumes", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return volume.ListResponse{}, err
	}

	var volumes volume.ListResponse
	err = json.NewDecoder(resp.Body).Decode(&volumes)
	return volumes, err
}
