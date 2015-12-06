package lib

import (
	"encoding/json"

	"github.com/docker/docker/api/types"
)

// ContainerInspect returns the all the container information.
func (cli *Client) ContainerInspect(containerID string) (types.ContainerJSON, error) {
	serverResp, err := cli.get("/containers/"+containerID+"/json", nil, nil)
	if err != nil {
		return types.ContainerJSON{}, err
	}
	defer ensureReaderClosed(serverResp)

	var response types.ContainerJSON
	json.NewDecoder(serverResp.body).Decode(&response)
	return response, err
}
