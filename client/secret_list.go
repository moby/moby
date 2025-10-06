package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// SecretList returns the list of secrets.
func (cli *Client) SecretList(ctx context.Context, options SecretListOptions) ([]swarm.Secret, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)
	resp, err := cli.get(ctx, "/secrets", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var secrets []swarm.Secret
	err = json.NewDecoder(resp.Body).Decode(&secrets)
	return secrets, err
}
