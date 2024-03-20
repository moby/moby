package client // import "github.com/docker/docker/client"

import (
	"context"
)

// ContainerLogsClear clears the logs of a container.
func (cli *Client) ContainerLogsClear(ctx context.Context, containerID string) error {
	resp, err := cli.post(ctx, "/containers/"+containerID+"/logs/clear", nil, nil, nil)
	ensureReaderClosed(resp)
	return err
}
