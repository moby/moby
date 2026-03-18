package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmGetUnlockKeyResult contains the swarm unlock key.
type SwarmGetUnlockKeyResult struct {
	Key string
}

// SwarmGetUnlockKey retrieves the swarm's unlock key.
func (cli *Client) SwarmGetUnlockKey(ctx context.Context) (SwarmGetUnlockKeyResult, error) {
	resp, err := cli.get(ctx, "/swarm/unlockkey", nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SwarmGetUnlockKeyResult{}, err
	}

	var response swarm.UnlockKeyResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return SwarmGetUnlockKeyResult{Key: response.UnlockKey}, err
}
