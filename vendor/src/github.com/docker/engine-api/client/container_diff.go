package client

import (
	"encoding/json"
	"net/url"

	"github.com/docker/engine-api/types"
)

// ContainerDiff shows differences in a container filesystem since it was started.
func (cli *Client) ContainerDiff(containerID string) ([]types.ContainerChange, error) {
	var changes []types.ContainerChange

	serverResp, err := cli.get("/containers/"+containerID+"/changes", url.Values{}, nil)
	if err != nil {
		return changes, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&changes)
	ensureReaderClosed(serverResp)
	return changes, err
}
