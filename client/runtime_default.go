package client

import (
	"net/url"

	"golang.org/x/net/context"
)

// RuntimeDefault sets the default runtime
func (cli *Client) RuntimeDefault(ctx context.Context, runtime string) error {
	query := url.Values{}
	query.Set("runtime", runtime)

	resp, err := cli.post(ctx, "/runtimes/"+runtime+"/default", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
