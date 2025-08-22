package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/system"
)

// DiskUsage requests the current data usage from the daemon
func (cli *Client) DiskUsage(ctx context.Context, options DiskUsageOptions) (system.DiskUsage, error) {
	var query url.Values
	if len(options.Types) > 0 {
		query = url.Values{}
		for _, t := range options.Types {
			query.Add("type", string(t))
		}
	}

	resp, err := cli.get(ctx, "/system/df", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return system.DiskUsage{}, err
	}

	var du system.DiskUsage
	if err := json.NewDecoder(resp.Body).Decode(&du); err != nil {
		return system.DiskUsage{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}
	return du, nil
}
