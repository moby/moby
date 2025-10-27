package client

import "context"

// ContainerUnPauseOptions holds options for [Client.ContainerUnpause].
type ContainerUnPauseOptions struct {
	// Add future optional parameters here.
}

// ContainerUnPauseResult holds the result of [Client.ContainerUnpause],
type ContainerUnPauseResult struct {
	// Add future fields here.
}

// ContainerUnpause resumes the process execution within a container.
func (cli *Client) ContainerUnpause(ctx context.Context, containerID string, options ContainerUnPauseOptions) (ContainerUnPauseResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerUnPauseResult{}, err
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/unpause", nil, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerUnPauseResult{}, err
	}
	return ContainerUnPauseResult{}, nil
}
