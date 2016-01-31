package client

import (
	"github.com/docker/engine-api/types/container"
)

// ContainerUpdate updates resources of a container
func (cli *Client) ContainerUpdate(containerID string, updateConfig container.UpdateConfig) error {
	resp, err := cli.post("/containers/"+containerID+"/update", nil, updateConfig, nil)
	ensureReaderClosed(resp)
	return err
}
