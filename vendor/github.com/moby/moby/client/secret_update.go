package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// SecretUpdateOptions holds options for updating a secret.
type SecretUpdateOptions struct {
	Version swarm.Version
	Spec    swarm.SecretSpec
}

type SecretUpdateResult struct{}

// SecretUpdate attempts to update a secret.
func (cli *Client) SecretUpdate(ctx context.Context, id string, options SecretUpdateOptions) (SecretUpdateResult, error) {
	id, err := trimID("secret", id)
	if err != nil {
		return SecretUpdateResult{}, err
	}
	query := url.Values{}
	query.Set("version", options.Version.String())
	resp, err := cli.post(ctx, "/secrets/"+id+"/update", query, options.Spec, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SecretUpdateResult{}, err
	}
	return SecretUpdateResult{}, nil
}
