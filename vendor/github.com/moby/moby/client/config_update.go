package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigUpdate attempts to update a config
func (cli *Client) ConfigUpdate(ctx context.Context, id string, version swarm.Version, config swarm.ConfigSpec) error {
	id, err := trimID("config", id)
	if err != nil {
		return err
	}
	if err := cli.NewVersionError(ctx, "1.30", "config update"); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("version", version.String())
	resp, err := cli.post(ctx, "/configs/"+id+"/update", query, config, nil)
	defer ensureReaderClosed(resp)
	return err
}
