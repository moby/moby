package client

import (
	"encoding/json"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"golang.org/x/net/context"
)

// ContainerUpdate updates resources of a container
func (cli *Client) ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (types.ContainerUpdateResponse, error) {
	var response types.ContainerUpdateResponse
	serverResp, err := cli.post(ctx, "/containers/"+containerID+"/update", nil, updateConfig, nil)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&response)

	ensureReaderClosed(serverResp)
	return response, err
}
