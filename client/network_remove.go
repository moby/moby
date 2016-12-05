package client

import (
	"net/url"

	"golang.org/x/net/context"
)

// NetworkRemove removes an existent network from the docker host.
func (cli *Client) NetworkRemove(ctx context.Context, networkID string) error {
	resp, err := cli.delete(ctx, "/networks/"+url.QueryEscape(networkID), nil, nil)
	ensureReaderClosed(resp)
	return err
}
