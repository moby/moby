package lib

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
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
func (cli *Client) NetworkConnect(networkID, containerID string) error {
	nc := types.NetworkConnect{Container: containerID}
	resp, err := cli.post("/networks/"+networkID+"/connect", nil, nc, nil)
	ensureReaderClosed(resp)
	return err
}

// NetworkDisconnect disconnects a container from an existent network in the docker host.
func (cli *Client) NetworkDisconnect(networkID, containerID string) error {
	nc := types.NetworkConnect{Container: containerID}
	resp, err := cli.post("/networks/"+networkID+"/disconnect", nil, nc, nil)
	ensureReaderClosed(resp)
	return err
}

// NetworkList returns the list of networks configured in the docker host.
func (cli *Client) NetworkList() ([]types.NetworkResource, error) {
	var networkResources []types.NetworkResource
	resp, err := cli.get("/networks", nil, nil)
	if err != nil {
		return networkResources, err
	}
	defer ensureReaderClosed(resp)
	err = json.NewDecoder(resp.body).Decode(&networkResources)
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
	defer ensureReaderClosed(resp)
	err = json.NewDecoder(resp.body).Decode(&networkResource)
	return networkResource, err
}
