package client

import (
	"context"

	"github.com/moby/moby/api/types/volume"
)

// VolumeInspectResult holds the result from the [Client.VolumeInspect] method.
type VolumeInspectResult struct {
	Raw    []byte
	Volume volume.Volume
}

// VolumeInspect returns the information about a specific volume in the docker host.
func (cli *Client) VolumeInspect(ctx context.Context, volumeID string) (VolumeInspectResult, error) {
	volumeID, err := trimID("volume", volumeID)
	if err != nil {
		return VolumeInspectResult{}, err
	}

	resp, err := cli.get(ctx, "/volumes/"+volumeID, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumeInspectResult{}, err
	}

	var out VolumeInspectResult
	out.Raw, err = decodeWithRaw(resp, &out.Volume)
	return out, err
}
