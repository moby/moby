package client

import (
	"context"
)

// StackDelete returns the details of a Stack
func (cli *Client) StackDelete(ctx context.Context, id string) error {

	headers := map[string][]string{
		"version": {cli.settings.Version},
	}

	resp, err := cli.delete(ctx, "/stacks/"+id, nil, headers)
	ensureReaderClosed(resp)
	return err
}
