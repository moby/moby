package client

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// ContainersPrune requests the daemon to delete unused data
func (cli *Client) ContainersPrune(ctx context.Context, cfg types.ContainersPruneConfig) (types.ContainersPruneReport, error) {
	var report types.ContainersPruneReport

	serverResp, err := cli.post(ctx, "/containers/prune", nil, cfg, nil)
	if err != nil {
		return report, err
	}
	defer ensureReaderClosed(serverResp)

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
