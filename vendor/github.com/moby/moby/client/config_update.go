package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigUpdate attempts to update a config
func (cli *Client) ConfigUpdate(ctx context.Context, vID SwarmVersionedID, config swarm.ConfigSpec) error {
	id, err := trimID("config", vID.ID)
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("version", vID.Version.String())
	resp, err := cli.post(ctx, "/configs/"+id+"/update", query, config, nil)
	defer ensureReaderClosed(resp)
	return err
}
