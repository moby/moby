package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// NodeUpdate updates a Node.
func (cli *Client) NodeUpdate(ctx context.Context, nodeID string, version swarm.Version, node swarm.NodeSpec) error {
	nodeID, err := trimID("node", nodeID)
	if err != nil {
		return err
	}

	query := url.Values{}
	query.Set("version", version.String())
	resp, err := cli.post(ctx, "/nodes/"+nodeID+"/update", query, node, nil)
	defer ensureReaderClosed(resp)
	return err
}
