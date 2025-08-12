package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// SecretUpdate attempts to update a secret.
func (cli *Client) SecretUpdate(ctx context.Context, id string, version swarm.Version, secret swarm.SecretSpec) error {
	id, err := trimID("secret", id)
	if err != nil {
		return err
	}
	if err := cli.NewVersionError(ctx, "1.25", "secret update"); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("version", version.String())
	resp, err := cli.post(ctx, "/secrets/"+id+"/update", query, secret, nil)
	defer ensureReaderClosed(resp)
	return err
}
