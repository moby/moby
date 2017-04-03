package client

import (
	"io"
	"net/url"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// PluginLoad loads a plugin
func (cli *Client) PluginLoad(ctx context.Context, input io.Reader) (types.PluginLoadResponse, error) {
	v := url.Values{}

	// set the type of the data request
	headers := map[string][]string{"Content-Type": {"application/x-tar"}}

	resp, err := cli.postRaw(ctx, "/plugins/load", v, input, headers)
	if err != nil {
		return types.PluginLoadResponse{}, err
	}

	return types.PluginLoadResponse{
		Body: resp.body,
		JSON: resp.header.Get("Content-Type") == "application/json",
	}, nil
}
