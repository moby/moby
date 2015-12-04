package lib

import (
	"net/url"
	"strconv"
)

// ContainerRestart stops and starts a container again.
// It makes the daemon to wait for the container to be up again for
// a specific amount of time, given the timeout.
func (cli *Client) ContainerRestart(containerID string, timeout int) error {
	var query url.Values
	query.Set("t", strconv.Itoa(timeout))
	resp, err := cli.POST("/containers"+containerID+"/restart", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
