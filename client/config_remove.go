package client

import "golang.org/x/net/context"

// ConfigRemove removes a Config.
func (cli *Client) ConfigRemove(ctx context.Context, id string) error {
	resp, err := cli.delete(ctx, "/configs/"+id, nil, nil)
	ensureReaderClosed(resp)
	return err
}
