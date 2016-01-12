package client

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
)

type configWrapper struct {
	*container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

// ContainerCreate creates a new container based in the given configuration.
// It can be associated with a name, but it's not mandatory.
func (cli *Client) ContainerCreate(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (types.ContainerCreateResponse, error) {
	var response types.ContainerCreateResponse
	query := url.Values{}
	if containerName != "" {
		query.Set("name", containerName)
	}

	body := configWrapper{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}

	serverResp, err := cli.post("/containers/create", query, body, nil)
	if err != nil {
		if serverResp != nil && serverResp.statusCode == 404 && strings.Contains(err.Error(), "No such image") {
			return response, imageNotFoundError{config.Image}
		}
		return response, err
	}

	if serverResp.statusCode == 404 && strings.Contains(err.Error(), "No such image") {
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
