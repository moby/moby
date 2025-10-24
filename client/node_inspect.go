package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/moby/moby/api/types/swarm"
)

// NodeInspectOptions holds parameters to inspect nodes with.
type NodeInspectOptions struct{}

type NodeInspectResult struct {
	Node swarm.Node
	Raw  json.RawMessage
}

// NodeInspect returns the node information.
func (cli *Client) NodeInspect(ctx context.Context, nodeID string, options NodeInspectOptions) (NodeInspectResult, error) {
	nodeID, err := trimID("node", nodeID)
	if err != nil {
		return NodeInspectResult{}, err
	}
	resp, err := cli.get(ctx, "/nodes/"+nodeID, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return NodeInspectResult{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NodeInspectResult{}, err
	}

	var response swarm.Node
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&response)
	return NodeInspectResult{Node: response, Raw: body}, err
}
