package client

import (
	"context"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmUnlock unlocks locked swarm.
func (cli *Client) SwarmUnlock(ctx context.Context, req swarm.UnlockRequest) error {
	resp, err := cli.post(ctx, "/swarm/unlock", nil, req, nil)
	defer ensureReaderClosed(resp)
	return err
}
