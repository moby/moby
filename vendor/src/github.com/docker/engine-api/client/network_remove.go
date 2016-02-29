package client

// NetworkRemove removes an existent network from the docker host.
func (cli *Client) NetworkRemove(networkID string) error {
	resp, err := cli.delete("/networks/"+networkID, nil, nil)
	ensureReaderClosed(resp)
	return err
}
