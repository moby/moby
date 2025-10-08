package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// NodeList returns the list of nodes.
func (cli *Client) NodeList(ctx context.Context, options NodeListOptions) ([]swarm.Node, error) {
	query := url.Values{}
	options.Filters.updateURLValues(query)
	resp, err := cli.get(ctx, "/nodes", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var nodes []swarm.Node
	err = json.NewDecoder(resp.Body).Decode(&nodes)
	return nodes, err
}
