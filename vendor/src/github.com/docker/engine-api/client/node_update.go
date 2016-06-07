package client

import (
	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

// NodeUpdate updates a Node.
func (cli *Client) NodeUpdate(ctx context.Context, nodeID string, node swarm.Node) error {
	resp, err := cli.post(ctx, "/nodes/"+nodeID+"/update", nil, node, nil)
	ensureReaderClosed(resp)
	return err
}
