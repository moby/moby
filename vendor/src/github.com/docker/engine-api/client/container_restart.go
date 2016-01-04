package client

import (
	"net/url"
	"strconv"
)

// ContainerRestart stops and starts a container again.
// It makes the daemon to wait for the container to be up again for
// a specific amount of time, given the timeout.
func (cli *Client) ContainerRestart(containerID string, timeout int) error {
	query := url.Values{}
	query.Set("t", strconv.Itoa(timeout))
	resp, err := cli.post("/containers/"+containerID+"/restart", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
