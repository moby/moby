package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
)

// ConfigCreate creates a new config.
func (cli *Client) ConfigCreate(ctx context.Context, config swarm.ConfigSpec) (types.ConfigCreateResponse, error) {
	var response types.ConfigCreateResponse
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return response, err
	}
	if err := versioned.NewVersionError("1.30", "config create"); err != nil {
		return response, err
	}
	resp, err := versioned.post(ctx, "/configs/create", nil, config, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)
	return response, err
}
