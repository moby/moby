package client

import (
	"net/url"

	"golang.org/x/net/context"
)

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	query := url.Values{}
	if force {
		query.Set("force", "1")
	}
	resp, err := cli.delete(ctx, "/volumes/"+volumeID, query, nil)
	ensureReaderClosed(resp)
	return err
}
