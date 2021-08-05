package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
)

// VolumeUsage requests the current volume usage from the daemon.
func (cli *Client) VolumeUsage(ctx context.Context) ([]*types.Volume, error) {
	serverResp, err := cli.get(ctx, "/volumes/usage", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var du []*types.Volume
	if err := json.NewDecoder(serverResp.body).Decode(&du); err != nil {
		return nil, fmt.Errorf("Error retrieving volume usage: %v", err)
	}
	return du, nil
}
