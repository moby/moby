package client

import "context"

type SecretRemoveOptions struct {
	// Add future optional parameters here
}

type SecretRemoveResult struct {
	// Add future fields here
}

// SecretRemove removes a secret.
func (cli *Client) SecretRemove(ctx context.Context, id string, options SecretRemoveOptions) (SecretRemoveResult, error) {
	id, err := trimID("secret", id)
	if err != nil {
		return SecretRemoveResult{}, err
	}
	resp, err := cli.delete(ctx, "/secrets/"+id, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SecretRemoveResult{}, err
	}
	return SecretRemoveResult{}, nil
}
