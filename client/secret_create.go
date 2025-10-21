package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/swarm"
)

// SecretCreateOptions holds options for creating a secret.
type SecretCreateOptions struct {
	Spec swarm.SecretSpec
}

// SecretCreateResult holds the result from the [Client.SecretCreate] method.
type SecretCreateResult struct {
	ID string
}

// SecretCreate creates a new secret.
func (cli *Client) SecretCreate(ctx context.Context, options SecretCreateOptions) (SecretCreateResult, error) {
	resp, err := cli.post(ctx, "/secrets/create", nil, options.Spec, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SecretCreateResult{}, err
	}

	var out swarm.ConfigCreateResponse
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return SecretCreateResult{}, err
	}
	return SecretCreateResult{ID: out.ID}, nil
}
