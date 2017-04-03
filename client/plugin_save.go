package client

import (
	"io"
	"net/http"
	"net/url"

	"golang.org/x/net/context"
)

// PluginSave retreives a plugin from docker as an io.ReadCloser.
// The caller is expected to store the plugin and close the stream.
func (cli *Client) PluginSave(ctx context.Context, plugin string) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("plugin", plugin)

	resp, err := cli.get(ctx, "/plugins/save", query, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return nil, pluginNotFoundError{plugin}
		}
		return nil, err
	}
	return resp.body, err
}
