package client

import (
	"context"
	"net/url"
	"strconv"
)

// ContainerRestart stops, and starts a container again.
// It makes the daemon wait for the container to be up again for
// a specific amount of time, given the timeout.
func (cli *Client) ContainerRestart(ctx context.Context, containerID string, options ContainerStopOptions) error {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return err
	}

	query := url.Values{}
	if options.Timeout != nil {
		query.Set("t", strconv.Itoa(*options.Timeout))
	}
	if options.Signal != "" {
		query.Set("signal", options.Signal)
	}
	resp, err := cli.post(ctx, "/containers/"+containerID+"/restart", query, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
