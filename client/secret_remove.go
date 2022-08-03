package client // import "github.com/docker/docker/client"

import "context"

// SecretRemove removes a secret.
func (cli *Client) SecretRemove(ctx context.Context, id string) error {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return err
	}
	if err := versioned.NewVersionError("1.25", "secret remove"); err != nil {
		return err
	}
	resp, err := versioned.delete(ctx, "/secrets/"+id, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
