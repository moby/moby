package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client/pkg/versions"
)

// NodeUpdateOptions holds parameters to update nodes with.
type NodeUpdateOptions struct {
	Version swarm.Version
	Spec    swarm.NodeSpec
}

type NodeUpdateResult struct{}

// NodeUpdateRequest holds the request body for API >= v1.53
type NodeUpdateRequest struct {
	Version uint64         `json:"version"`
	Spec    swarm.NodeSpec `json:"spec"`
}

// NodeUpdate updates a Node.
func (cli *Client) NodeUpdate(ctx context.Context, nodeID string, options NodeUpdateOptions) (NodeUpdateResult, error) {
	nodeID, err := trimID("node", nodeID)
	if err != nil {
		return NodeUpdateResult{}, err
	}

	var (
		query url.Values
		body  io.Reader
	)

	if versions.GreaterThanOrEqualTo(cli.version, "1.53") {
		req := NodeUpdateRequest{
			Version: options.Version.Index,
			Spec:    options.Spec,
		}

		buf, err := json.Marshal(req)
		if err != nil {
			return NodeUpdateResult{}, err
		}
		body = bytes.NewReader(buf)
	} else {
		query = url.Values{}
		query.Set("version", options.Version.String())

		// For older API versions, spec goes in body directly
		buf, err := json.Marshal(options.Spec)
		if err != nil {
			return NodeUpdateResult{}, err
		}
		body = bytes.NewReader(buf)
	}

	resp, err := cli.post(ctx, "/nodes/"+nodeID+"/update", query, body, nil)
	defer ensureReaderClosed(resp)
	return NodeUpdateResult{}, err
}
