package client

import (
	"net/url"
	"strconv"

	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

// ServiceUpdate updates a Service.
func (cli *Client) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec) error {
	query := url.Values{}
	query.Set("version", strconv.FormatUint(version.Index, 10))
	resp, err := cli.post(ctx, "/services/"+serviceID+"/update", query, service, nil)
	ensureReaderClosed(resp)
	return err
}
