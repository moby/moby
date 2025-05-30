package client

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types/swarm"
)

// ConfigCreate creates a new config.
func (cli *Client) ConfigCreate(ctx context.Context, config swarm.ConfigSpec) (swarm.ConfigCreateResponse, error) {
	var response swarm.ConfigCreateResponse
	if err := cli.NewVersionError(ctx, "1.30", "config create"); err != nil {
		return response, err
	}
	resp, err := cli.post(ctx, "/configs/create", nil, config, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	return response, err
}
