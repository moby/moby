package client // import "github.com/moby/moby/client"

import (
	"context"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/swarm"
)

// NodeUpdate updates a Node.
func (cli *Client) NodeUpdate(ctx context.Context, nodeID string, version swarm.Version, node swarm.NodeSpec) error {
	query := url.Values{}
	query.Set("version", strconv.FormatUint(version.Index, 10))
	resp, err := cli.post(ctx, "/nodes/"+nodeID+"/update", query, node, nil)
	ensureReaderClosed(resp)
	return err
}
