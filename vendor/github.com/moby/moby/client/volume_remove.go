package client

import (
	"context"
	"net/url"
)

// VolumeRemoveOptions holds options for [Client.VolumeRemove].
type VolumeRemoveOptions struct {
	// Force the removal of the volume
	Force bool
}

// VolumeRemoveResult holds the result of [Client.VolumeRemove],
type VolumeRemoveResult struct {
	// Add future fields here.
}

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(ctx context.Context, volumeID string, options VolumeRemoveOptions) (VolumeRemoveResult, error) {
	volumeID, err := trimID("volume", volumeID)
	if err != nil {
		return VolumeRemoveResult{}, err
	}

	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}
	resp, err := cli.delete(ctx, "/volumes/"+volumeID, query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumeRemoveResult{}, err
	}
	return VolumeRemoveResult{}, nil
}
