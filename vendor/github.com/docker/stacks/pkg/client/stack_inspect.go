package client

import (
	"context"
	"encoding/json"

	"github.com/docker/stacks/pkg/types"
)

// StackInspect returns the details of a Stack
func (cli *Client) StackInspect(ctx context.Context, id string) (types.Stack, error) {

	headers := map[string][]string{
		"version": {cli.settings.Version},
	}

	var response types.Stack
	resp, err := cli.get(ctx, "/stacks/"+id, nil, headers)
	if err != nil {
		return response, wrapResponseError(err, resp, "stack", id)
	}

	err = json.NewDecoder(resp.body).Decode(&response)

	ensureReaderClosed(resp)
	return response, err
}
