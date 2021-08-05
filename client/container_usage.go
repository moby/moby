package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
)

// ContainerUsage requests the current container usage from the daemon.
func (cli *Client) ContainerUsage(ctx context.Context) ([]*types.ContainerUsage, error) {
	serverResp, err := cli.get(ctx, "/containers/usage", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var us []*types.ContainerUsage
	if err := json.NewDecoder(serverResp.body).Decode(&us); err != nil {
		return nil, fmt.Errorf("Error retrieving container usage: %v", err)
	}
	return us, nil
}
