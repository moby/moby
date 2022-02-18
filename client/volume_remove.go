package client // import "github.com/moby/moby/client"

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/versions"
)

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	query := url.Values{}
	if versions.GreaterThanOrEqualTo(cli.version, "1.25") {
		if force {
			query.Set("force", "1")
		}
	}
	resp, err := cli.delete(ctx, "/volumes/"+volumeID, query, nil)
	defer ensureReaderClosed(resp)
	return wrapResponseError(err, resp, "volume", volumeID)
}
