package client

import (
	"context"
	"encoding/json"

	"github.com/docker/stacks/pkg/types"
)

// ParseComposeInput takes a compose file and returns a parsed StackCreate object
func (cli *Client) ParseComposeInput(ctx context.Context, input types.ComposeInput) (*types.StackCreate, error) {

	headers := map[string][]string{
		"version": {cli.settings.Version},
	}

	var response types.StackCreate
	resp, err := cli.post(ctx, "/parsecompose", nil, input, headers)
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)

	ensureReaderClosed(resp)
	return &response, err
}
