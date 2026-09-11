package client

import (
	"context"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmUnlockOptions specifies options for unlocking a swarm.
type SwarmUnlockOptions struct {
	Key string
}

// SwarmUnlockResult represents the result of unlocking a swarm.
type SwarmUnlockResult struct{}

// SwarmUnlock unlocks locked swarm.
func (cli *Client) SwarmUnlock(ctx context.Context, options SwarmUnlockOptions) (SwarmUnlockResult, error) {
	resp, err := cli.post(ctx, "/swarm/unlock", nil, nil, swarm.UnlockRequest{
		UnlockKey: options.Key,
	})
	defer ensureReaderClosed(resp)
	return SwarmUnlockResult{}, err
}
