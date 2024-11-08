package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/url"
)

// VolumeExport export a local volume from the docker host.
func (cli *Client) VolumeExport(ctx context.Context, volumeID string) (io.ReadCloser, error) {
	serverResp, err := cli.get(ctx, "/volumes/"+volumeID+"/export", url.Values{}, nil)
	if err != nil {
		return nil, err
	}

	return serverResp.body, nil
}
