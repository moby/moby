package client

import (
	"context"
	"net/url"
)

// ContainerStartOptions holds options for [Client.ContainerStart].
type ContainerStartOptions struct {
	CheckpointID  string
	CheckpointDir string
}

// ContainerStartResult holds the result of [Client.ContainerStart],
type ContainerStartResult struct {
	// Add future fields here.
}

// ContainerStart sends a request to the docker daemon to start a container.
func (cli *Client) ContainerStart(ctx context.Context, containerID string, options ContainerStartOptions) (ContainerStartResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerStartResult{}, err
	}

	query := url.Values{}
	if options.CheckpointID != "" {
		query.Set("checkpoint", options.CheckpointID)
	}
	if options.CheckpointDir != "" {
		query.Set("checkpoint-dir", options.CheckpointDir)
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/start", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerStartResult{}, err
	}
	return ContainerStartResult{}, nil
}
