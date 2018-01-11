package client

import (
	"golang.org/x/net/context"
	"net/url"
)

// VolumeRename rename a volume on the docker host.
func (cli *Client) VolumeRename(ctx context.Context, volume, newVolume string) error {
	query := url.Values{}
	query.Set("name", volume)
	query.Set("newName", newVolume)

	resp, err := cli.post(ctx, "/volumes/rename", query, nil, nil)
	if err != nil {
		return err
	}
	ensureReaderClosed(resp)
	return err
}
