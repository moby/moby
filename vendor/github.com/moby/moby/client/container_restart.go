package client

import (
	"context"
	"net/url"
	"strconv"
)

// ContainerRestartOptions holds options for [Client.ContainerRestart].
type ContainerRestartOptions struct {
	// Signal (optional) is the signal to send to the container to (gracefully)
	// stop it before forcibly terminating the container with SIGKILL after the
	// timeout expires. If no value is set, the default (SIGTERM) is used.
	Signal string `json:",omitempty"`

	// Timeout (optional) is the timeout (in seconds) to wait for the container
	// to stop gracefully before forcibly terminating it with SIGKILL.
	//
	// - Use nil to use the default timeout (10 seconds).
	// - Use '-1' to wait indefinitely.
	// - Use '0' to not wait for the container to exit gracefully, and
	//   immediately proceeds to forcibly terminating the container.
	// - Other positive values are used as timeout (in seconds).
	Timeout *int `json:",omitempty"`
}

// ContainerRestartResult holds the result of [Client.ContainerRestart],
type ContainerRestartResult struct {
	// Add future fields here.
}

// ContainerRestart stops, and starts a container again.
// It makes the daemon wait for the container to be up again for
// a specific amount of time, given the timeout.
func (cli *Client) ContainerRestart(ctx context.Context, containerID string, options ContainerRestartOptions) (ContainerRestartResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerRestartResult{}, err
	}

	query := url.Values{}
	if options.Timeout != nil {
		query.Set("t", strconv.Itoa(*options.Timeout))
	}
	if options.Signal != "" {
		query.Set("signal", options.Signal)
	}
	resp, err := cli.post(ctx, "/containers/"+containerID+"/restart", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerRestartResult{}, err
	}
	return ContainerRestartResult{}, nil
}
