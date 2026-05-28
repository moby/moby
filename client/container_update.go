package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/container"
)

// ContainerUpdateOptions holds options for [Client.ContainerUpdate].
type ContainerUpdateOptions struct {
	Resources     *container.Resources
	RestartPolicy *container.RestartPolicy
}

// ContainerUpdateResult is the result from updating a container.
type ContainerUpdateResult struct {
	// Warnings encountered when updating the container.
	Warnings []string
}

// ContainerUpdate updates the resources of a container.
func (cli *Client) ContainerUpdate(ctx context.Context, containerID string, options ContainerUpdateOptions) (ContainerUpdateResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerUpdateResult{}, err
	}

	updateConfig := container.UpdateConfig{}
	if options.Resources != nil {
		updateConfig.Resources = *options.Resources
	}
	if options.RestartPolicy != nil {
		updateConfig.RestartPolicy = *options.RestartPolicy
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/update", nil, updateConfig, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerUpdateResult{}, err
	}

	var response container.UpdateResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return ContainerUpdateResult{Warnings: response.Warnings}, err
}
