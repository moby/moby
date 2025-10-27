package client

import (
	"context"
	"net/url"
	"strconv"
)

// ContainerStopOptions holds the options for [Client.ContainerStop].
type ContainerStopOptions struct {
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

// ContainerStopResult holds the result of [Client.ContainerStop],
type ContainerStopResult struct {
	// Add future fields here.
}

// ContainerStop stops a container. In case the container fails to stop
// gracefully within a time frame specified by the timeout argument,
// it is forcefully terminated (killed).
//
// If the timeout is nil, the container's StopTimeout value is used, if set,
// otherwise the engine default. A negative timeout value can be specified,
// meaning no timeout, i.e. no forceful termination is performed.
func (cli *Client) ContainerStop(ctx context.Context, containerID string, options ContainerStopOptions) (ContainerStopResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerStopResult{}, err
	}

	query := url.Values{}
	if options.Timeout != nil {
		query.Set("t", strconv.Itoa(*options.Timeout))
	}
	if options.Signal != "" {
		query.Set("signal", options.Signal)
	}
	resp, err := cli.post(ctx, "/containers/"+containerID+"/stop", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerStopResult{}, err
	}
	return ContainerStopResult{}, nil
}
