package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types/network"
)

// NetworkCreate creates a new network in the docker host.
func (cli *Client) NetworkCreate(ctx context.Context, name string, options network.NetworkCreate) (network.CreateResponse, error) {
	networkCreateRequest := network.CreateRequest{
		NetworkCreate: options,
		Name:          name,
	}
	var response network.CreateResponse
	serverResp, err := cli.post(ctx, "/networks/create", nil, networkCreateRequest, nil)
	if err != nil {
		return response, err
	}

	json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}
