package client

import (
	"context"
	"net/url"
)

// ContainerRemoveOptions holds parameters to remove containers.
type ContainerRemoveOptions struct {
	RemoveVolumes bool
	RemoveLinks   bool
	Force         bool
}

// ContainerRemoveResult holds the result of [Client.ContainerRemove],
type ContainerRemoveResult struct {
	// Add future fields here.
}

// ContainerRemove kills and removes a container from the docker host.
func (cli *Client) ContainerRemove(ctx context.Context, containerID string, options ContainerRemoveOptions) (ContainerRemoveResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerRemoveResult{}, err
	}

	query := url.Values{}
	if options.RemoveVolumes {
		query.Set("v", "1")
	}
	if options.RemoveLinks {
		query.Set("link", "1")
	}

	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.delete(ctx, "/containers/"+containerID, query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerRemoveResult{}, err
	}
	return ContainerRemoveResult{}, nil
}
