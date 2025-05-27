package client // import "github.com/docker/docker/client"

import (
	"context"
)

// PluginSet modifies settings for an existing plugin
func (cli *Client) PluginSet(ctx context.Context, name string, args []string) error {
	name, err := trimID("plugin", name)
	if err != nil {
		return err
	}

	resp, err := cli.post(ctx, "/plugins/"+name+"/set", nil, args, nil)
	ensureReaderClosed(resp)
	return err
}
