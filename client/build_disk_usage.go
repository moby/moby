package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
)

// BuildDiskUsage requests the current volume data usage from the daemon.
func (cli *Client) BuildDiskUsage(ctx context.Context) ([]*types.BuildCache, error) {
	serverResp, err := cli.get(ctx, "/builds/usage", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var du []*types.BuildCache
	if err := json.NewDecoder(serverResp.body).Decode(&du); err != nil {
		return nil, fmt.Errorf("Error retrieving build-cache disk usage: %v", err)
	}
	return du, nil
}
