package lib

// ContainerStart sends a request to the docker daemon to start a container.
func (cli *Client) ContainerStart(containerID string) error {
	resp, err := cli.post("/containers/"+containerID+"/start", nil, nil, nil)
	ensureReaderClosed(resp)
	return err
}
