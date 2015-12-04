package lib

// ContainerUnpause resumes the process execution within a container
func (cli *Client) ContainerUnpause(containerID string) error {
	resp, err := cli.POST("/containers/"+containerID+"/unpause", nil, nil, nil)
	ensureReaderClosed(resp)
	return err
}
