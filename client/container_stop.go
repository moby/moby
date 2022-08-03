package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
)

// ContainerStop stops a container. In case the container fails to stop
// gracefully within a time frame specified by the timeout argument,
// it is forcefully terminated (killed).
//
// If the timeout is nil, the container's StopTimeout value is used, if set,
// otherwise the engine default. A negative timeout value can be specified,
// meaning no timeout, i.e. no forceful termination is performed.
func (cli *Client) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
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
	resp, err := versioned.post(ctx, "/containers/"+containerID+"/stop", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
