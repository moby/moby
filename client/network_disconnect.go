package client

import (
	"context"

	"github.com/moby/moby/api/types/network"
)

// NetworkDisconnectOptions represents the data to be used to disconnect a container
// from the network.
type NetworkDisconnectOptions struct {
	Container string
	Force     bool
}

// NetworkDisconnectResult represents the result of a NetworkDisconnect operation.
type NetworkDisconnectResult struct {
	// Currently empty; placeholder for future fields.
}

// NetworkDisconnect disconnects a container from an existent network in the docker host.
func (cli *Client) NetworkDisconnect(ctx context.Context, networkID string, options NetworkDisconnectOptions) (NetworkDisconnectResult, error) {
	networkID, err := trimID("network", networkID)
	if err != nil {
		return NetworkDisconnectResult{}, err
	}

	containerID, err := trimID("container", options.Container)
	if err != nil {
		return NetworkDisconnectResult{}, err
	}

	req := network.DisconnectRequest{
		Container: containerID,
		Force:     options.Force,
	}
	resp, err := cli.post(ctx, "/networks/"+networkID+"/disconnect", nil, req, nil)
	defer ensureReaderClosed(resp)
	return NetworkDisconnectResult{}, err
}
