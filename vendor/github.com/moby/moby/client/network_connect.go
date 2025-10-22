package client

import (
	"context"

	"github.com/moby/moby/api/types/network"
)

// NetworkConnect connects a container to an existent network in the docker host.
func (cli *Client) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	networkID, err := trimID("network", networkID)
	if err != nil {
		return err
	}

	containerID, err = trimID("container", containerID)
	if err != nil {
		return err
	}

	req := network.ConnectRequest{
		Container:      containerID,
		EndpointConfig: config,
	}
	resp, err := cli.post(ctx, "/networks/"+networkID+"/connect", nil, req, nil)
	defer ensureReaderClosed(resp)
	return err
}
