package client

import (
	"net/url"

	"golang.org/x/net/context"
)

// ServiceRemove kills and removes a service.
func (cli *Client) ServiceRemove(ctx context.Context, serviceID string) error {
	resp, err := cli.delete(ctx, "/services/"+url.QueryEscape(serviceID), nil, nil)
	ensureReaderClosed(resp)
	return err
}
