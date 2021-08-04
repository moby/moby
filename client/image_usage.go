package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
)

// ImageUsage requests the current image usage from the daemon.
func (cli *Client) ImageUsage(ctx context.Context) ([]*types.ImageUsage, error) {
	serverResp, err := cli.get(ctx, "/images/usage", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var du []*types.ImageSummary
	if err := json.NewDecoder(serverResp.body).Decode(&du); err != nil {
		return nil, fmt.Errorf("Error retrieving image usage: %v", err)
	}
	return du, nil
}
