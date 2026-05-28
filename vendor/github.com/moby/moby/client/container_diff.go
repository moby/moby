package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/container"
)

// ContainerDiff shows differences in a container filesystem since it was started.
func (cli *Client) ContainerDiff(ctx context.Context, containerID string, options ContainerDiffOptions) (ContainerDiffResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerDiffResult{}, err
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/changes", url.Values{}, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerDiffResult{}, err
	}

	var changes []container.FilesystemChange
	err = json.NewDecoder(resp.Body).Decode(&changes)
	if err != nil {
		return ContainerDiffResult{}, err
	}
	return ContainerDiffResult{Changes: changes}, err
}
