package client

import "context"

// SecretRemove removes a secret.
func (cli *Client) SecretRemove(ctx context.Context, id string) error {
	id, err := trimID("secret", id)
	if err != nil {
		return err
	}
	resp, err := cli.delete(ctx, "/secrets/"+id, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
