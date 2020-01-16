package client // import "github.com/moby/moby/client"

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/swarm"
)

// ConfigCreate creates a new Config.
func (cli *Client) ConfigCreate(ctx context.Context, config swarm.ConfigSpec) (types.ConfigCreateResponse, error) {
	var response types.ConfigCreateResponse
	if err := cli.NewVersionError("1.30", "config create"); err != nil {
		return response, err
	}
	resp, err := cli.post(ctx, "/configs/create", nil, config, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)
	return response, err
}
