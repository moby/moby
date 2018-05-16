package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"

	"github.com/docker/docker/api/types/swarm"
)

// SwarmGetUnlockKey retrieves the swarm's unlock key.
func (cli *Client) SwarmGetUnlockKey(ctx context.Context) (swarm.UnlockKeyResponse, error) {
	serverResp, err := cli.get(ctx, "/swarm/unlockkey", nil, nil)
	if err != nil {
		return swarm.UnlockKeyResponse{}, err
	}

	var response swarm.UnlockKeyResponse
	err = json.NewDecoder(serverResp.body).Decode(&response)
	ensureReaderClosed(serverResp)
	return response, err
}
