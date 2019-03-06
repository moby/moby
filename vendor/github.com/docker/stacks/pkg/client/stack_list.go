package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/stacks/pkg/types"

	"github.com/docker/docker/api/types/filters"
)

// StackList returns the list of Stacks on the server
func (cli *Client) StackList(ctx context.Context, options types.StackListOptions) ([]types.Stack, error) {

	headers := map[string][]string{
		"version": {cli.settings.Version},
	}

	query := url.Values{}
	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return nil, err
		}

		query.Set("filters", filterJSON)
	}

	var response []types.Stack
	resp, err := cli.get(ctx, "/stacks", query, headers)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)

	ensureReaderClosed(resp)
	return response, err
}
