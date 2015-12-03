package lib

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
)

// ContainerDiff shows differences in a container filesystem since it was started.
func (cli *Client) ContainerDiff(containerID string) ([]types.ContainerChange, error) {
	var changes []types.ContainerChange

	serverResp, err := cli.GET("/containers/"+containerID+"/changes", url.Values{}, nil)
	if err != nil {
		return changes, err
	}
	defer serverResp.body.Close()

	if err := json.NewDecoder(serverResp.body).Decode(&changes); err != nil {
		return changes, err
	}

	return changes, nil
}
