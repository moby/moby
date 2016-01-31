package client

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
)

// NetworkCreate creates a new network in the docker host.
func (cli *Client) NetworkCreate(options types.NetworkCreate) (types.NetworkCreateResponse, error) {
	var response types.NetworkCreateResponse
	serverResp, err := cli.post("/networks/create", nil, options, nil)
	if err != nil {
		return response, err
	}

	json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}

// NetworkRemove removes an existent network from the docker host.
func (cli *Client) NetworkRemove(networkID string) error {
	resp, err := cli.delete("/networks/"+networkID, nil, nil)
	ensureReaderClosed(resp)
	return err
}

// NetworkConnect connects a container to an existent network in the docker host.
func (cli *Client) NetworkConnect(networkID, containerID string, config *network.EndpointSettings) error {
	nc := types.NetworkConnect{
		Container:      containerID,
		EndpointConfig: config,
	}
	resp, err := cli.post("/networks/"+networkID+"/connect", nil, nc, nil)
	ensureReaderClosed(resp)
	return err
}

// NetworkDisconnect disconnects a container from an existent network in the docker host.
func (cli *Client) NetworkDisconnect(networkID, containerID string, force bool) error {
	nd := types.NetworkDisconnect{Container: containerID, Force: force}
	resp, err := cli.post("/networks/"+networkID+"/disconnect", nil, nd, nil)
	ensureReaderClosed(resp)
	return err
}

// NetworkList returns the list of networks configured in the docker host.
func (cli *Client) NetworkList(options types.NetworkListOptions) ([]types.NetworkResource, error) {
	query := url.Values{}
	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToParam(options.Filters)
		if err != nil {
			return nil, err
		}

		query.Set("filters", filterJSON)
	}
	var networkResources []types.NetworkResource
	resp, err := cli.get("/networks", query, nil)
	if err != nil {
		return networkResources, err
	}
	err = json.NewDecoder(resp.body).Decode(&networkResources)
	ensureReaderClosed(resp)
	return networkResources, err
}

// NetworkInspect returns the information for a specific network configured in the docker host.
func (cli *Client) NetworkInspect(networkID string) (types.NetworkResource, error) {
	var networkResource types.NetworkResource
	resp, err := cli.get("/networks/"+networkID, nil, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return networkResource, networkNotFoundError{networkID}
		}
		return networkResource, err
	}
	err = json.NewDecoder(resp.body).Decode(&networkResource)
	ensureReaderClosed(resp)
	return networkResource, err
}
