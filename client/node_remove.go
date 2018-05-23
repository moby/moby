package client // import "github.com/docker/docker/client"

import (
	"net/url"

	"context"
)

// NodeRemoveOptions holds parameters to remove nodes with.
type NodeRemoveOptions struct {
	Force bool
}

// NodeRemove removes a Node.
func (cli *Client) NodeRemove(ctx context.Context, nodeID string, options NodeRemoveOptions) error {
	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.delete(ctx, "/nodes/"+nodeID, query, nil)
	ensureReaderClosed(resp)
	return wrapResponseError(err, resp, "node", nodeID)
}
