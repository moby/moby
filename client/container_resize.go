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

// ContainerResize changes the size of the pseudo-TTY for a container.
func (cli *Client) ContainerResize(ctx context.Context, containerID string, options ContainerResizeOptions) error {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return err
	}
	return cli.resize(ctx, "/containers/"+containerID, options.Height, options.Width)
}

// ContainerExecResize changes the size of the tty for an exec process running inside a container.
func (cli *Client) ContainerExecResize(ctx context.Context, execID string, options ContainerResizeOptions) error {
	execID, err := trimID("exec", execID)
	if err != nil {
		return err
	}
	return cli.resize(ctx, "/exec/"+execID, options.Height, options.Width)
}

func (cli *Client) resize(ctx context.Context, basePath string, height, width uint) error {
	// FIXME(thaJeztah): the API / backend accepts uint32, but container.ResizeOptions uses uint.
	query := url.Values{}
	query.Set("h", strconv.FormatUint(uint64(height), 10))
	query.Set("w", strconv.FormatUint(uint64(width), 10))

	resp, err := cli.post(ctx, basePath+"/resize", query, nil, nil)
	defer ensureReaderClosed(resp)
	return err
}
