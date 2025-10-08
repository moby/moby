package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigCreateOptions holds options for creating a config.
type ConfigCreateOptions struct {
	Config swarm.ConfigSpec
}

// ConfigCreateResult holds the result from the ConfigCreate method.
type ConfigCreateResult struct {
	Response swarm.ConfigCreateResponse
}

// ConfigCreate creates a new config.
func (cli *Client) ConfigCreate(ctx context.Context, options ConfigCreateOptions) (ConfigCreateResult, error) {
	var out ConfigCreateResult

	resp, err := cli.post(ctx, "/configs/create", nil, options.Config, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return out, err
	}

	err = json.NewDecoder(resp.Body).Decode(&out.Response)
	return out, err
}
