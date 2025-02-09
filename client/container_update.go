package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types/container"
)

// ContainerUpdate updates the resources of a container.
func (cli *Client) ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.ContainerUpdateOKBody, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return container.ContainerUpdateOKBody{}, err
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/update", nil, updateConfig, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return container.ContainerUpdateOKBody{}, err
	}

	var response container.ContainerUpdateOKBody
	err = json.NewDecoder(resp.Body).Decode(&response)
	return response, err
}
