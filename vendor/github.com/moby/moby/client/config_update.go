package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigUpdateOptions holds options for updating a config.
type ConfigUpdateOptions struct {
	Version swarm.Version
	Spec    swarm.ConfigSpec
}

type ConfigUpdateResult struct{}

// ConfigUpdate attempts to update a config
func (cli *Client) ConfigUpdate(ctx context.Context, id string, options ConfigUpdateOptions) (ConfigUpdateResult, error) {
	id, err := trimID("config", id)
	if err != nil {
		return ConfigUpdateResult{}, err
	}
	query := url.Values{}
	query.Set("version", options.Version.String())
	resp, err := cli.post(ctx, "/configs/"+id+"/update", query, options.Spec, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ConfigUpdateResult{}, err
	}
	return ConfigUpdateResult{}, nil
}
