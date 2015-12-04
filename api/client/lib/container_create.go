package lib

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/runconfig"
)

// ContainerCreate creates a new container based in the given configuration.
// It can be associated with a name, but it's not mandatory.
func (cli *Client) ContainerCreate(config *runconfig.ContainerConfigWrapper, containerName string) (types.ContainerCreateResponse, error) {
	var (
		query    url.Values
		response types.ContainerCreateResponse
	)
	if containerName != "" {
		query.Set("name", containerName)
	}

	serverResp, err := cli.POST("/containers/create", query, config, nil)
	if err != nil {
		if serverResp != nil && serverResp.statusCode == 404 && strings.Contains(err.Error(), config.Image) {
			return response, imageNotFoundError{config.Image}
		}
		return response, err
	}

	if serverResp.statusCode == 404 && strings.Contains(err.Error(), config.Image) {
		return response, imageNotFoundError{config.Image}
	}

	if err != nil {
		return response, err
	}
	defer ensureReaderClosed(serverResp)

	if err := json.NewDecoder(serverResp.body).Decode(&response); err != nil {
		return response, err
	}

	return response, nil
}
