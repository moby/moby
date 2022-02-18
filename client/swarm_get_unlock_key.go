package client // import "github.com/moby/moby/client"

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types"
)

// SwarmGetUnlockKey retrieves the swarm's unlock key.
func (cli *Client) SwarmGetUnlockKey(ctx context.Context) (types.SwarmUnlockKeyResponse, error) {
	serverResp, err := cli.get(ctx, "/swarm/unlockkey", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return types.SwarmUnlockKeyResponse{}, err
	}

	var response types.SwarmUnlockKeyResponse
	err = json.NewDecoder(serverResp.body).Decode(&response)
	return response, err
}
