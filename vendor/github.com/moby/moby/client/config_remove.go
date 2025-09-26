package client

import "context"

// ConfigRemove removes a config.
func (cli *Client) ConfigRemove(ctx context.Context, id string) error {
	id, err := trimID("config", id)
	if err != nil {
		return err
	}
	resp, err := cli.delete(ctx, "/configs/"+id, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
