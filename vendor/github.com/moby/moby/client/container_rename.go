package client

import (
	"context"
	"net/url"
)

// ContainerRename changes the name of a given container.
func (cli *Client) ContainerRename(ctx context.Context, containerID, newContainerName string) error {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return err
	}

	query := url.Values{}
	query.Set("name", newContainerName)
	resp, err := cli.post(ctx, "/containers/"+containerID+"/rename", query, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
