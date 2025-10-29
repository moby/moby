package client

import (
	"context"

	"github.com/moby/moby/api/types/network"
)

// NetworkConnectOptions represents the data to be used to connect a container to the
// network.
type NetworkConnectOptions struct {
	Container      string
	EndpointConfig *network.EndpointSettings
}

// NetworkConnectResult represents the result of a NetworkConnect operation.
type NetworkConnectResult struct {
	// Currently empty; placeholder for future fields.
}

// NetworkConnect connects a container to an existent network in the docker host.
func (cli *Client) NetworkConnect(ctx context.Context, networkID string, options NetworkConnectOptions) (NetworkConnectResult, error) {
	networkID, err := trimID("network", networkID)
	if err != nil {
		return NetworkConnectResult{}, err
	}

	containerID, err := trimID("container", options.Container)
	if err != nil {
		return NetworkConnectResult{}, err
	}

	nc := network.ConnectRequest{
		Container:      containerID,
		EndpointConfig: options.EndpointConfig,
	}
	resp, err := cli.post(ctx, "/networks/"+networkID+"/connect", nil, nc, nil)
	defer ensureReaderClosed(resp)
	return NetworkConnectResult{}, err
}
