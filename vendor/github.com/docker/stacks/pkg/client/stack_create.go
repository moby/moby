package client

import (
	"context"
	"encoding/json"

	"github.com/docker/stacks/pkg/types"
)

// StackCreate creates a new Stack
func (cli *Client) StackCreate(ctx context.Context, stack types.StackCreate, options types.StackCreateOptions) (types.StackCreateResponse, error) {
	headers := map[string][]string{
		"version": {cli.settings.Version},
	}

	if options.EncodedRegistryAuth != "" {
		headers["X-Registry-Auth"] = []string{options.EncodedRegistryAuth}
	}

	var response types.StackCreateResponse
	resp, err := cli.post(ctx, "/stacks", nil, stack, headers)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)

	ensureReaderClosed(resp)
	return response, err
}
