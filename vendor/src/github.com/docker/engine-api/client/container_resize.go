package client

import (
	"net/url"
	"strconv"

	"github.com/docker/engine-api/types"
)

// ContainerResize changes the size of the tty for a container.
func (cli *Client) ContainerResize(options types.ResizeOptions) error {
	return cli.resize("/containers/"+options.ID, options.Height, options.Width)
}

// ContainerExecResize changes the size of the tty for an exec process running inside a container.
func (cli *Client) ContainerExecResize(options types.ResizeOptions) error {
	return cli.resize("/exec/"+options.ID, options.Height, options.Width)
}

func (cli *Client) resize(basePath string, height, width int) error {
	query := url.Values{}
	query.Set("h", strconv.Itoa(height))
	query.Set("w", strconv.Itoa(width))

	resp, err := cli.post(basePath+"/resize", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
