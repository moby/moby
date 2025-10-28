package client

import (
	"context"
	"net/url"
	"strconv"
)

// ContainerResizeOptions holds parameters to resize a TTY.
// It can be used to resize container TTYs and
// exec process TTYs too.
type ContainerResizeOptions struct {
	Height uint
	Width  uint
}

// ContainerResizeResult holds the result of [Client.ContainerResize],
type ContainerResizeResult struct {
	// Add future fields here.
}

// ContainerResize changes the size of the pseudo-TTY for a container.
func (cli *Client) ContainerResize(ctx context.Context, containerID string, options ContainerResizeOptions) (ContainerResizeResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerResizeResult{}, err
	}
	// FIXME(thaJeztah): the API / backend accepts uint32, but container.ResizeOptions uses uint.
	query := url.Values{}
	query.Set("h", strconv.FormatUint(uint64(options.Height), 10))
	query.Set("w", strconv.FormatUint(uint64(options.Width), 10))

	resp, err := cli.post(ctx, "/containers/"+containerID+"/resize", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerResizeResult{}, err
	}
	return ContainerResizeResult{}, nil
}

// ExecResizeOptions holds options for resizing a container exec TTY.
type ExecResizeOptions ContainerResizeOptions

// ExecResizeResult holds the result of resizing a container exec TTY.
type ExecResizeResult struct {
}

// ExecResize changes the size of the tty for an exec process running inside a container.
func (cli *Client) ExecResize(ctx context.Context, execID string, options ExecResizeOptions) (ExecResizeResult, error) {
	execID, err := trimID("exec", execID)
	if err != nil {
		return ExecResizeResult{}, err
	}
	// FIXME(thaJeztah): the API / backend accepts uint32, but container.ResizeOptions uses uint.
	query := url.Values{}
	query.Set("h", strconv.FormatUint(uint64(options.Height), 10))
	query.Set("w", strconv.FormatUint(uint64(options.Width), 10))

	resp, err := cli.post(ctx, "/exec/"+execID+"/resize", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ExecResizeResult{}, err
	}
	return ExecResizeResult{}, nil

}
