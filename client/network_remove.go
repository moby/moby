package client

import (
	"context"
)

// NetworkRemoveOptions specifies options for removing a network.
type NetworkRemoveOptions struct {
	// No options currently; placeholder for future use.
}

// NetworkRemoveResult represents the result of a network removal operation.
type NetworkRemoveResult struct {
	// No fields currently; placeholder for future use.
}

// NetworkRemove removes an existent network from the docker host.
func (cli *Client) NetworkRemove(ctx context.Context, networkID string, options NetworkRemoveOptions) (NetworkRemoveResult, error) {
	networkID, err := trimID("network", networkID)
	if err != nil {
		return NetworkRemoveResult{}, err
	}
	resp, err := cli.delete(ctx, "/networks/"+networkID, nil, nil)
	defer ensureReaderClosed(resp)
	return NetworkRemoveResult{}, err
}
