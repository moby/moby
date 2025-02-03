package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/container"
)

// ContainerDiff shows differences in a container filesystem since it was started.
func (cli *Client) ContainerDiff(ctx context.Context, containerID string) ([]container.FilesystemChange, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return nil, err
	}

	serverResp, err := cli.get(ctx, "/containers/"+containerID+"/changes", url.Values{}, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var changes []container.FilesystemChange
	err = json.NewDecoder(serverResp.body).Decode(&changes)
	if err != nil {
		return nil, err
	}
	return changes, err
}
