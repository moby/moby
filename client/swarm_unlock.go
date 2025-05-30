package client

import (
	"context"

	"github.com/docker/docker/api/types/swarm"
)

// SwarmUnlock unlocks locked swarm.
func (cli *Client) SwarmUnlock(ctx context.Context, req swarm.UnlockRequest) error {
	resp, err := cli.post(ctx, "/swarm/unlock", nil, req, nil)
	ensureReaderClosed(resp)
	return err
}
