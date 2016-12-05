package client

import (
	"net/url"

	"golang.org/x/net/context"
)

// PluginDisable disables a plugin
func (cli *Client) PluginDisable(ctx context.Context, name string) error {
	resp, err := cli.post(ctx, "/plugins/"+url.QueryEscape(name)+"/disable", nil, nil, nil)
	ensureReaderClosed(resp)
	return err
}
