package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
)

// ContainerRestart stops and starts a container again.
// It makes the daemon wait for the container to be up again for
// a specific amount of time, given the timeout.
func (cli *Client) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return err
	}
	query := url.Values{}
	if options.Timeout != nil {
		query.Set("t", strconv.Itoa(*options.Timeout))
	}
	if options.Signal != "" && versions.GreaterThanOrEqualTo(versioned.version, "1.42") {
		query.Set("signal", options.Signal)
	}
	resp, err := versioned.post(ctx, "/containers/"+containerID+"/restart", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
