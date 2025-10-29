package client

import (
	"context"
	"net/url"
)

// ContainerRenameOptions represents the options for renaming a container.
type ContainerRenameOptions struct {
	NewName string
}

// ContainerRenameResult represents the result of a container rename operation.
type ContainerRenameResult struct {
	// This struct can be expanded in the future if needed
}

// ContainerRename changes the name of a given container.
func (cli *Client) ContainerRename(ctx context.Context, containerID string, options ContainerRenameOptions) (ContainerRenameResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerRenameResult{}, err
	}

	query := url.Values{}
	query.Set("name", options.NewName)
	resp, err := cli.post(ctx, "/containers/"+containerID+"/rename", query, nil, nil)
	defer ensureReaderClosed(resp)
	return ContainerRenameResult{}, err
}
