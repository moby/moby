package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
)

// NetworkCreate creates a new network in the docker host.
func (cli *Client) NetworkCreate(ctx context.Context, name string, options types.NetworkCreate) (types.NetworkCreateResponse, error) {
	networkCreateRequest := types.NetworkCreateRequest{
		NetworkCreate: options,
		Name:          name,
	}
	if versions.LessThan(cli.version, "1.44") {
		networkCreateRequest.CheckDuplicate = true //nolint:staticcheck // ignore SA1019: CheckDuplicate is deprecated since API v1.44.
	}

	var response types.NetworkCreateResponse
	serverResp, err := cli.post(ctx, "/networks/create", nil, networkCreateRequest, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&response)
	return response, err
}
