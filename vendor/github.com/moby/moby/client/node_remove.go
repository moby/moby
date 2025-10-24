package client

import (
	"context"
	"net/url"
)

// NodeRemoveOptions holds parameters to remove nodes with.
type NodeRemoveOptions struct {
	Force bool
}
type NodeRemoveResult struct{}

// NodeRemove removes a Node.
func (cli *Client) NodeRemove(ctx context.Context, nodeID string, options NodeRemoveOptions) (NodeRemoveResult, error) {
	nodeID, err := trimID("node", nodeID)
	if err != nil {
		return NodeRemoveResult{}, err
	}

	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.delete(ctx, "/nodes/"+nodeID, query, nil)
	defer ensureReaderClosed(resp)
	return NodeRemoveResult{}, err
}
