package client

import (
	"net/url"

	"github.com/docker/docker/api/types/versions"
	"golang.org/x/net/context"
)

// NetworkRemove removes an existent network from the docker host.
func (cli *Client) NetworkRemove(ctx context.Context, networkID string, force bool) error {
	query := url.Values{}
	if versions.GreaterThanOrEqualTo(cli.version, "1.28") {
		if force {
			query.Set("force", "1")
		}
	}
	resp, err := cli.delete(ctx, "/networks/"+networkID, query, nil)
	ensureReaderClosed(resp)
	return err
}
