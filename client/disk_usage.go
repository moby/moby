package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/docker/docker/api/types"
)

// DiskUsage requests the current data usage from the daemon
func (cli *Client) DiskUsage(ctx context.Context, options types.DiskUsageOptions) (types.DiskUsage, error) {
	var query url.Values
	if len(options.Types) > 0 {
		query = url.Values{}
		for _, t := range options.Types {
			query.Add("type", string(t))
		}
	}

	serverResp, err := cli.get(ctx, "/system/df", query, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return types.DiskUsage{}, err
	}

	var du types.DiskUsage
	if err := json.NewDecoder(serverResp.body).Decode(&du); err != nil {
		return types.DiskUsage{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}
	return du, nil
}
