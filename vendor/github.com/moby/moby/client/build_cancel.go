package client

import (
	"context"
	"net/url"
)

// BuildCancel requests the daemon to cancel the ongoing build request
// with the given id.
func (cli *Client) BuildCancel(ctx context.Context, id string) error {
	query := url.Values{}
	query.Set("id", id)

	resp, err := cli.post(ctx, "/build/cancel", query, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
