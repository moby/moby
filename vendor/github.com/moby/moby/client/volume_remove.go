package client

import (
	"context"
	"net/url"
)

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	volumeID, err := trimID("volume", volumeID)
	if err != nil {
		return err
	}

	query := url.Values{}
	if force {
		query.Set("force", "1")
	}
	resp, err := cli.delete(ctx, "/volumes/"+volumeID, query, nil)
	defer ensureReaderClosed(resp)
	return err
}
