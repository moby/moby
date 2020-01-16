package client // import "github.com/moby/moby/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/filters"
	volumetypes "github.com/moby/moby/api/types/volume"
)

// VolumeList returns the volumes configured in the docker host.
func (cli *Client) VolumeList(ctx context.Context, filter filters.Args) (volumetypes.VolumeListOKBody, error) {
	var volumes volumetypes.VolumeListOKBody
	query := url.Values{}

	if filter.Len() > 0 {
		//nolint:staticcheck // ignore SA1019 for old code
		filterJSON, err := filters.ToParamWithVersion(cli.version, filter)
		if err != nil {
			return volumes, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get(ctx, "/volumes", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return volumes, err
	}

	err = json.NewDecoder(resp.body).Decode(&volumes)
	return volumes, err
}
