package client

import "context"

// ContainerUnpauseOptions holds options for [Client.ContainerUnpause].
type ContainerUnpauseOptions struct {
	// Add future optional parameters here.
}

// ContainerUnpauseResult holds the result of [Client.ContainerUnpause],
type ContainerUnpauseResult struct {
	// Add future fields here.
}

// ContainerUnpause resumes the process execution within a container.
func (cli *Client) ContainerUnpause(ctx context.Context, containerID string, options ContainerUnpauseOptions) (ContainerUnpauseResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerUnpauseResult{}, err
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/unpause", nil, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerUnpauseResult{}, err
	}
	return ContainerUnpauseResult{}, nil
}
