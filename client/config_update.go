package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigUpdateOptions holds options for updating a config.
type ConfigUpdateOptions struct {
	Config swarm.ConfigSpec
}

type ConfigUpdateResult struct{}

// ConfigUpdate attempts to update a config
func (cli *Client) ConfigUpdate(ctx context.Context, vID SwarmVersionedID, options ConfigUpdateOptions) (ConfigUpdateResult, error) {
	id, err := trimID("config", vID.ID)
	if err != nil {
		return ConfigUpdateResult{}, err
	}
	query := url.Values{}
	query.Set("version", vID.Version.String())
	resp, err := cli.post(ctx, "/configs/"+id+"/update", query, options.Config, nil)
	defer ensureReaderClosed(resp)
	return ConfigUpdateResult{}, err
}
