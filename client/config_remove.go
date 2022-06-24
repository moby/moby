package client // import "github.com/docker/docker/client"

import "context"

// ConfigRemove removes a config.
func (cli *Client) ConfigRemove(ctx context.Context, id string) error {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return err
	}
	if err := versioned.NewVersionError("1.30", "config remove"); err != nil {
		return err
	}
	resp, err := versioned.delete(ctx, "/configs/"+id, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
