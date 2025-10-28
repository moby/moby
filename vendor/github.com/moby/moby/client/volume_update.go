package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/volume"
)

// VolumeUpdateOptions holds options for [Client.VolumeUpdate].
type VolumeUpdateOptions struct {
	Version swarm.Version
	// Spec is the ClusterVolumeSpec to update the volume to.
	Spec *volume.ClusterVolumeSpec `json:"Spec,omitempty"`
}

// VolumeUpdateResult holds the result of [Client.VolumeUpdate],
type VolumeUpdateResult struct {
	// Add future fields here.
}

// VolumeUpdate updates a volume. This only works for Cluster Volumes, and
// only some fields can be updated.
func (cli *Client) VolumeUpdate(ctx context.Context, volumeID string, options VolumeUpdateOptions) (VolumeUpdateResult, error) {
	volumeID, err := trimID("volume", volumeID)
	if err != nil {
		return VolumeUpdateResult{}, err
	}

	query := url.Values{}
	query.Set("version", options.Version.String())

	resp, err := cli.put(ctx, "/volumes/"+volumeID, query, options, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumeUpdateResult{}, err
	}
	return VolumeUpdateResult{}, nil
}
