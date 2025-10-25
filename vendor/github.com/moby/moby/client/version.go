package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types"
)

// ServerVersionResult contains information about the Docker server host.
type ServerVersionResult struct {
	Version types.Version
}

// ServerVersion returns information of the Docker server host.
func (cli *Client) ServerVersion(ctx context.Context) (ServerVersionResult, error) {
	resp, err := cli.get(ctx, "/version", nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ServerVersionResult{}, err
	}

	var version types.Version
	err = json.NewDecoder(resp.Body).Decode(&version)
	return ServerVersionResult{Version: version}, err
}
