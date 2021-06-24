package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
)

// ContainerDiskUsage requests the current container data usage from the daemon.
func (cli *Client) ContainerDiskUsage(ctx context.Context) ([]*types.Container, error) {
	serverResp, err := cli.get(ctx, "/containers/usage", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var du []*types.Container
	if err := json.NewDecoder(serverResp.body).Decode(&du); err != nil {
		return nil, fmt.Errorf("Error retrieving container disk usage: %v", err)
	}
	return du, nil
}
