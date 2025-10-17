package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/moby/moby/api/types/volume"
)

// VolumeInspectResult holds the result from the [Client.VolumeInspect] method.
type VolumeInspectResult struct {
	Volume volume.Volume
}

// VolumeInspect returns the information about a specific volume in the docker host.
func (cli *Client) VolumeInspect(ctx context.Context, volumeID string) (VolumeInspectResult, error) {
	vol, _, err := cli.VolumeInspectWithRaw(ctx, volumeID)
	return vol, err
}

// VolumeInspectWithRaw returns the information about a specific volume in the docker host and its raw representation
func (cli *Client) VolumeInspectWithRaw(ctx context.Context, volumeID string) (VolumeInspectResult, []byte, error) {
	volumeID, err := trimID("volume", volumeID)
	if err != nil {
		return VolumeInspectResult{}, nil, err
	}

	resp, err := cli.get(ctx, "/volumes/"+volumeID, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumeInspectResult{}, nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return VolumeInspectResult{}, nil, err
	}

	var vol volume.Volume
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&vol)
	return VolumeInspectResult{Volume: vol}, body, err
}
