package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"
	"strconv"
)

// ResizeOptions holds parameters to resize a tty.
// It can be used to resize container ttys and
// exec process ttys too.
type ResizeOptions struct {
	Height uint
	Width  uint
}

// ContainerResize changes the size of the tty for a container.
func (cli *Client) ContainerResize(ctx context.Context, containerID string, options ResizeOptions) error {
	return cli.resize(ctx, "/containers/"+containerID, options.Height, options.Width)
}

// ContainerExecResize changes the size of the tty for an exec process running inside a container.
func (cli *Client) ContainerExecResize(ctx context.Context, execID string, options ResizeOptions) error {
	return cli.resize(ctx, "/exec/"+execID, options.Height, options.Width)
}

func (cli *Client) resize(ctx context.Context, basePath string, height, width uint) error {
	query := url.Values{}
	query.Set("h", strconv.Itoa(int(height)))
	query.Set("w", strconv.Itoa(int(width)))

	resp, err := cli.post(ctx, basePath+"/resize", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
