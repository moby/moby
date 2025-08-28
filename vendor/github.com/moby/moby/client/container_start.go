package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/container"
)

// ContainerStart sends a request to the docker daemon to start a container.
func (cli *Client) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return err
	}

	query := url.Values{}
	if options.CheckpointID != "" {
		query.Set("checkpoint", options.CheckpointID)
	}
	if options.CheckpointDir != "" {
		query.Set("checkpoint-dir", options.CheckpointDir)
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/start", query, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
