package client // import "github.com/moby/moby/client"

import (
	"context"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/swarm"
)

// ConfigUpdate attempts to update a config
func (cli *Client) ConfigUpdate(ctx context.Context, id string, version swarm.Version, config swarm.ConfigSpec) error {
	if err := cli.NewVersionError("1.30", "config update"); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("version", strconv.FormatUint(version.Index, 10))
	resp, err := cli.post(ctx, "/configs/"+id+"/update", query, config, nil)
	ensureReaderClosed(resp)
	return err
}
