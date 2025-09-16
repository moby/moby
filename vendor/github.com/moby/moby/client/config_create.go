package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigCreate creates a new config.
func (cli *Client) ConfigCreate(ctx context.Context, config swarm.ConfigSpec) (swarm.ConfigCreateResponse, error) {
	resp, err := cli.post(ctx, "/configs/create", nil, config, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return swarm.ConfigCreateResponse{}, err
	}

	var response swarm.ConfigCreateResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return response, err
}
