package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// NodeListOptions holds parameters to list nodes with.
type NodeListOptions struct {
	Filters Filters
}

type NodeListResult struct {
	Items []swarm.Node
}

// NodeList returns the list of nodes.
func (cli *Client) NodeList(ctx context.Context, options NodeListOptions) (NodeListResult, error) {
	query := url.Values{}
	options.Filters.updateURLValues(query)
	resp, err := cli.get(ctx, "/nodes", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return NodeListResult{}, err
	}

	var nodes []swarm.Node
	err = json.NewDecoder(resp.Body).Decode(&nodes)
	return NodeListResult{Items: nodes}, err
}
