package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/volume"
)

// VolumeUpdate updates a volume. This only works for Cluster Volumes, and
// only some fields can be updated.
func (cli *Client) VolumeUpdate(ctx context.Context, volumeID string, version swarm.Version, options volume.UpdateOptions) error {
	volumeID, err := trimID("volume", volumeID)
	if err != nil {
		return err
	}
	if err := cli.NewVersionError(ctx, "1.42", "volume update"); err != nil {
		return err
	}

	query := url.Values{}
	query.Set("version", version.String())

	resp, err := cli.put(ctx, "/volumes/"+volumeID, query, options, nil)
	defer ensureReaderClosed(resp)
	return err
}
