package client

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types/swarm"
)

// SwarmGetUnlockKey retrieves the swarm's unlock key.
func (cli *Client) SwarmGetUnlockKey(ctx context.Context) (swarm.UnlockKeyResponse, error) {
	resp, err := cli.get(ctx, "/swarm/unlockkey", nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return swarm.UnlockKeyResponse{}, err
	}

	var response swarm.UnlockKeyResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return response, err
}
