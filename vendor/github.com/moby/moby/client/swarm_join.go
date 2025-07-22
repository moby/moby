package client

import (
	"context"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmJoin joins the swarm.
func (cli *Client) SwarmJoin(ctx context.Context, req swarm.JoinRequest) error {
	resp, err := cli.post(ctx, "/swarm/join", nil, req, nil)
	defer ensureReaderClosed(resp)
	return err
}
