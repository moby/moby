package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// PluginEnable enables a plugin
func (cli *Client) PluginEnable(ctx context.Context, name string, options types.PluginEnableOptions) error {
	name, err := trimID("plugin", name)
	if err != nil {
		return err
	}
	query := url.Values{}
	if v := options.Timeout; v != 0 {
		if v < 0 {
			return errdefs.InvalidParameter(errors.New("invalid timeout: value must be positive"))
		}
		query.Set("timeout", strconv.Itoa(v))
	}
	resp, err := cli.post(ctx, "/plugins/"+name+"/enable", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
