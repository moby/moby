package lib

import (
	"net/url"

	"github.com/docker/docker/api/types"
)

// ContainerRestore restores a running container
func (cli *Client) ContainerRestore(containerID string, options types.CriuConfig, forceRestore bool) error {
	query := url.Values{}

	if forceRestore {
		query.Set("force", "1")
	}

	resp, err := cli.post("/containers/"+containerID+"/restore", query, options, nil)
	ensureReaderClosed(resp)

	return err
}
