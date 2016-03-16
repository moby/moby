package client

import (
	"fmt"
	"net/url"
)

// ContainerStart sends a request to the docker daemon to start a container.
func (cli *Client) ContainerStart(containerID string) error {
	resp, err := cli.post("/containers/"+containerID+"/start", nil, nil, nil)
	ensureReaderClosed(resp)
	return err
}

// ContainerStartWithCommand sends a request to the docker daemon to start a
// container, but allows the client to override the default command.
func (cli *Client) ContainerStartWithCommand(containerID string, cmd string) error {
	if cmd == "" {
		return fmt.Errorf("Command can not be empty")
	}

	query := url.Values{}
	query.Set("cmd", cmd)

	resp, err := cli.post("/containers/"+containerID+"/start", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
