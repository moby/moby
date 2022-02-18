package client // import "github.com/moby/moby/client"

import "context"

// ServiceRemove kills and removes a service.
func (cli *Client) ServiceRemove(ctx context.Context, serviceID string) error {
	resp, err := cli.delete(ctx, "/services/"+serviceID, nil, nil)
	defer ensureReaderClosed(resp)
	return wrapResponseError(err, resp, "service", serviceID)
}
