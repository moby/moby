package client

import (
	"context"
	"net/url"
)

// ContainerKillOptions holds options for [Client.ContainerKill].
type ContainerKillOptions struct {
	// Signal (optional) is the signal to send to the container to (gracefully)
	// stop it before forcibly terminating the container with SIGKILL after a
	// timeout. If no value is set, the default (SIGKILL) is used.
	Signal string `json:",omitempty"`
}

// ContainerKillResult holds the result of [Client.ContainerKill],
type ContainerKillResult struct {
	// Add future fields here.
}

// ContainerKill terminates the container process but does not remove the container from the docker host.
func (cli *Client) ContainerKill(ctx context.Context, containerID string, options ContainerKillOptions) (ContainerKillResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerKillResult{}, err
	}

	query := url.Values{}
	if options.Signal != "" {
		query.Set("signal", options.Signal)
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/kill", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerKillResult{}, err
	}
	return ContainerKillResult{}, nil
}
