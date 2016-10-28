package client

import (
	"encoding/json"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// BandwidthCreateRequest creates a new bandwidth for docker container in the docker host.
func (cli *Client) BandwidthCreateRequest(ctx context.Context, name string, options types.BandwidthCreateRequest) (types.BandwidthCreateResponse, error) {

	bandwidthCreateRequest := options

	var response types.BandwidthCreateResponse
	serverResp, err := cli.post(ctx, "/networks/bandwidth", nil, bandwidthCreateRequest, nil)
	if err != nil {
		return response, err
	}

	json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}
