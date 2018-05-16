package client // import "github.com/docker/docker/client"

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

// ContainerRemove kills and removes a container from the docker host.
func (cli *Client) ContainerRemove(ctx context.Context, containerID string, options ContainerRemoveOptions) error {
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
	ensureReaderClosed(resp)
	return wrapResponseError(err, resp, "container", containerID)
}
