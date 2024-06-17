package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
)

// NetworkCreate creates a new network in the docker host.
func (cli *Client) NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
	var response network.CreateResponse

	// Make sure we negotiated (if the client is configured to do so),
	// as code below contains API-version specific handling of options.
	//
	// Normally, version-negotiation (if enabled) would not happen until
	// the API request is made.
	if err := cli.checkVersion(ctx); err != nil {
		return response, err
	}

	networkCreateRequest := network.CreateRequest{
		CreateOptions: options,
		Name:          name,
	}
	if versions.LessThan(cli.version, "1.44") {
		enabled := true
		networkCreateRequest.CheckDuplicate = &enabled //nolint:staticcheck // ignore SA1019: CheckDuplicate is deprecated since API v1.44.
	}

	serverResp, err := cli.post(ctx, "/networks/create", nil, networkCreateRequest, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&response)
	return response, err
}
