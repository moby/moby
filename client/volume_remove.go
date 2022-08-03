package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"

	"github.com/docker/docker/api/types/versions"
)

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return err
	}
	query := url.Values{}
	if versions.GreaterThanOrEqualTo(versioned.version, "1.25") {
		if force {
			query.Set("force", "1")
		}
	}
	resp, err := versioned.delete(ctx, "/volumes/"+volumeID, query, nil)
	defer ensureReaderClosed(resp)
	return err
}
