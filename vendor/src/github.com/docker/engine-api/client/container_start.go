package client

import (
	"net/url"

	"golang.org/x/net/context"
)

// ContainerStart sends a request to the docker daemon to start a container.
func (cli *Client) ContainerStart(ctx context.Context, containerID string, checkpointID string) error {
	query := url.Values{}
	query.Set("checkpoint", checkpointID)

	resp, err := cli.post(ctx, "/containers/"+containerID+"/start", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
