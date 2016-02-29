package client

import (
	"encoding/json"

	"github.com/docker/engine-api/types"
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
