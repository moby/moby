package lib

import "net/url"

// ContainerKill terminates the container process but does not remove the container from the docker host.
func (cli *Client) ContainerKill(containerID, signal string) error {
	query := url.Values{}
	query.Set("signal", signal)

	resp, err := cli.post("/containers/"+containerID+"/kill", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
