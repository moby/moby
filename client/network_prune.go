package client

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// NetworksPrune requests the daemon to delete unused networks
func (cli *Client) NetworksPrune(ctx context.Context, cfg types.NetworksPruneConfig) (types.NetworksPruneReport, error) {
	var report types.NetworksPruneReport

	serverResp, err := cli.post(ctx, "/networks/prune", nil, cfg, nil)
	if err != nil {
		return report, err
	}
	defer ensureReaderClosed(serverResp)

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving network prune report: %v", err)
	}

	return report, nil
}
