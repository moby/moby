package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/system"
)

// Info returns information about the docker server.
func (cli *Client) Info(ctx context.Context) (system.Info, error) {
	var info system.Info
	resp, err := cli.get(ctx, "/info", url.Values{}, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return info, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return info, fmt.Errorf("Error reading remote info: %v", err)
	}

	return info, nil
}
