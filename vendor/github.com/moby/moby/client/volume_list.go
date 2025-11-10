package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/volume"
)

// VolumeListOptions holds parameters to list volumes.
type VolumeListOptions struct {
	Filters Filters
}

// VolumeListResult holds the result from the [Client.VolumeList] method.
type VolumeListResult struct {
	// List of volumes.
	Items []volume.Volume

	// Warnings that occurred when fetching the list of volumes.
	Warnings []string
}

// VolumeList returns the volumes configured in the docker host.
func (cli *Client) VolumeList(ctx context.Context, options VolumeListOptions) (VolumeListResult, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)
	resp, err := cli.get(ctx, "/volumes", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumeListResult{}, err
	}

	var apiResp volume.ListResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	if err != nil {
		return VolumeListResult{}, err
	}

	return VolumeListResult{
		Items:    apiResp.Volumes,
		Warnings: apiResp.Warnings,
	}, nil
}
