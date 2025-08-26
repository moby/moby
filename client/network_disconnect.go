package client

import (
	"context"
)

// NetworkDisconnect disconnects a container from an existent network in the docker host.
func (cli *Client) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
	networkID, err := trimID("network", networkID)
	if err != nil {
		return err
	}

	containerID, err = trimID("container", containerID)
	if err != nil {
		return err
	}

	nd := NetworkDisconnectOptions{
		Container: containerID,
		Force:     force,
	}
	resp, err := cli.post(ctx, "/networks/"+networkID+"/disconnect", nil, nd, nil)
	defer ensureReaderClosed(resp)
	return err
}
