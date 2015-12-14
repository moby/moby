package lib

import (
	"github.com/docker/docker/runconfig"
)

// ContainerCheckpoint checkpoints a running container
func (cli *Client) ContainerCheckpoint(containerID string, options runconfig.CriuConfig) error {
	resp, err := cli.post("/containers/"+containerID+"/checkpoint", nil, options, nil)
	ensureReaderClosed(resp)

	return err
}
