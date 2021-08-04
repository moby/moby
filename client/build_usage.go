package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
)

// BuildDiskUsage requests the current build-cache usage from the daemon.
func (cli *Client) BuildUsage(ctx context.Context) ([]*types.BuildCacheUsage, error) {
	serverResp, err := cli.get(ctx, "/builds/usage", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var us []*types.BuildCacheUsage
	if err := json.NewDecoder(serverResp.body).Decode(&us); err != nil {
		return nil, fmt.Errorf("Error retrieving build-cache usage: %v", err)
	}
	return us, nil
}
