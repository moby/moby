package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// NodeUpdateOptions holds parameters to update nodes with.
type NodeUpdateOptions struct {
	Version swarm.Version
	Spec    swarm.NodeSpec
}

type NodeUpdateResult struct{}

// NodeUpdate updates a Node.
func (cli *Client) NodeUpdate(ctx context.Context, nodeID string, options NodeUpdateOptions) (NodeUpdateResult, error) {
	nodeID, err := trimID("node", nodeID)
	if err != nil {
		return NodeUpdateResult{}, err
	}

	query := url.Values{}
	query.Set("version", options.Version.String())
	resp, err := cli.post(ctx, "/nodes/"+nodeID+"/update", query, options.Spec, nil)
	defer ensureReaderClosed(resp)
	return NodeUpdateResult{}, err
}
