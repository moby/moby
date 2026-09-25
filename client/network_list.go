package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/network"
)

// NetworkListResult holds the result from the [Client.NetworkList] method.
type NetworkListResult struct {
	Items []network.Summary
}

// NetworkList returns the list of networks configured in the docker host.
func (cli *Client) NetworkList(ctx context.Context, options NetworkListOptions) (NetworkListResult, error) {
	query := url.Values{}
	options.Filters.updateURLValues(query)
	resp, err := cli.get(ctx, "/networks", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return NetworkListResult{}, err
	}
	var res NetworkListResult
	err = json.NewDecoder(resp.Body).Decode(&res.Items)
	return res, err
}
