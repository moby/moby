package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
)

// ImageDiskUsage requests the current image data usage from the daemon.
func (cli *Client) ImageDiskUsage(ctx context.Context) ([]*types.ImageSummary, error) {
	serverResp, err := cli.get(ctx, "/images/usage", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var du []*types.ImageSummary
	if err := json.NewDecoder(serverResp.body).Decode(&du); err != nil {
		return nil, fmt.Errorf("Error retrieving image disk usage: %v", err)
	}
	return du, nil
}
