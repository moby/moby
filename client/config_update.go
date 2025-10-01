package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigUpdateOptions holds options for updating a config.
type ConfigUpdateOptions struct {
	Version swarm.Version
	Config  swarm.ConfigSpec
}

// ConfigUpdate attempts to update a config
func (cli *Client) ConfigUpdate(ctx context.Context, id string, options ConfigUpdateOptions) error {
	id, err := trimID("config", id)
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("version", options.Version.String())
	resp, err := cli.post(ctx, "/configs/"+id+"/update", query, options.Config, nil)
	defer ensureReaderClosed(resp)
	return err
}
