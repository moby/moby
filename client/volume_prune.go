package client

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// VolumesPrune requests the daemon to delete unused data
func (cli *Client) VolumesPrune(ctx context.Context, cfg types.VolumesPruneConfig) (types.VolumesPruneReport, error) {
	var report types.VolumesPruneReport

	serverResp, err := cli.post(ctx, "/volumes/prune", nil, cfg, nil)
	if err != nil {
		return report, err
	}
	defer ensureReaderClosed(serverResp)

	if err := json.NewDecoder(serverResp.body).Decode(&report); err != nil {
		return report, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	return report, nil
}
