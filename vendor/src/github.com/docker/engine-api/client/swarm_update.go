package client

import (
	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

// SwarmUpdate updates the Swarm.
func (cli *Client) SwarmUpdate(ctx context.Context, swarm swarm.Swarm) error {
	resp, err := cli.post(ctx, "/swarm/update", nil, swarm, nil)
	ensureReaderClosed(resp)
	return err
}
