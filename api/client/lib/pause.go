package lib

// ContainerPause pauses the main process of a given container without terminating it.
func (cli *Client) ContainerPause(containerID string) error {
	resp, err := cli.post("/containers/"+containerID+"/pause", nil, nil, nil)
	ensureReaderClosed(resp)
	return err
}
