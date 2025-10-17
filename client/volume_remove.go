package client

import (
	"context"
	"net/url"
)

// VolumeRemoveOptions holds optional parameters for volume removal.
type VolumeRemoveOptions struct {
	// Force the removal of the volume
	Force bool
}

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(ctx context.Context, volumeID string, options VolumeRemoveOptions) error {
	volumeID, err := trimID("volume", volumeID)
	if err != nil {
		return err
	}

	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}
	resp, err := cli.delete(ctx, "/volumes/"+volumeID, query, nil)
	defer ensureReaderClosed(resp)
	return err
}
