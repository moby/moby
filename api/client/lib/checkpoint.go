package lib

import (
	"github.com/docker/docker/api/types"
)

// ContainerCheckpoint checkpoints a running container
func (cli *Client) ContainerCheckpoint(containerID string, options types.CriuConfig) error {
	resp, err := cli.post("/containers/"+containerID+"/checkpoint", nil, options, nil)
	ensureReaderClosed(resp)

	return err
}
