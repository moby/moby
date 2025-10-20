package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigCreateOptions holds options for creating a config.
type ConfigCreateOptions struct {
	Spec swarm.ConfigSpec
}

// ConfigCreateResult holds the result from the ConfigCreate method.
type ConfigCreateResult struct {
	ID string
}

// ConfigCreate creates a new config.
func (cli *Client) ConfigCreate(ctx context.Context, options ConfigCreateOptions) (ConfigCreateResult, error) {
	resp, err := cli.post(ctx, "/configs/create", nil, options.Spec, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ConfigCreateResult{}, err
	}

	var out swarm.ConfigCreateResponse
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return ConfigCreateResult{}, err
	}
	return ConfigCreateResult{ID: out.ID}, nil
}
