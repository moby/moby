package client

import "context"

// ContainerPauseOptions holds options for [Client.ContainerPause].
type ContainerPauseOptions struct {
	// Add future optional parameters here.
}

// ContainerPauseResult holds the result of [Client.ContainerPause],
type ContainerPauseResult struct {
	// Add future fields here.
}

// ContainerPause pauses the main process of a given container without terminating it.
func (cli *Client) ContainerPause(ctx context.Context, containerID string, options ContainerPauseOptions) (ContainerPauseResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerPauseResult{}, err
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/pause", nil, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerPauseResult{}, err
	}
	return ContainerPauseResult{}, nil
}
