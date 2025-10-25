package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/system"
)

type InfoOptions struct {
	// No options currently; placeholder for future use
}

type SystemInfoResult struct {
	Info system.Info
}

// Info returns information about the docker server.
func (cli *Client) Info(ctx context.Context, options InfoOptions) (SystemInfoResult, error) {
	resp, err := cli.get(ctx, "/info", url.Values{}, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SystemInfoResult{}, err
	}

	var info system.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return SystemInfoResult{}, fmt.Errorf("Error reading remote info: %v", err)
	}

	return SystemInfoResult{Info: info}, nil
}
