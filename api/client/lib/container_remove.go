package lib

import (
	"net/url"

	"github.com/docker/docker/api/types"
)

// ContainerRemove kills and removes a container from the docker host.
func (cli *Client) ContainerRemove(options types.ContainerRemoveOptions) error {
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

	resp, err := cli.DELETE("/containers/"+options.ContainerID, query, nil)
	ensureReaderClosed(resp)
	return err
}
