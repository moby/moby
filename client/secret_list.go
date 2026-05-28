package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// SecretListOptions holds parameters to list secrets
type SecretListOptions struct {
	Filters Filters
}

// SecretListResult holds the result from the [client.SecretList] method.
type SecretListResult struct {
	Items []swarm.Secret
}

// SecretList returns the list of secrets.
func (cli *Client) SecretList(ctx context.Context, options SecretListOptions) (SecretListResult, error) {
	query := url.Values{}
	options.Filters.updateURLValues(query)

	resp, err := cli.get(ctx, "/secrets", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SecretListResult{}, err
	}

	var out SecretListResult
	err = json.NewDecoder(resp.Body).Decode(&out.Items)
	if err != nil {
		return SecretListResult{}, err
	}
	return out, nil
}
