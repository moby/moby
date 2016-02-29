package client

// VolumeRemove removes a volume from the docker host.
func (cli *Client) VolumeRemove(volumeID string) error {
	resp, err := cli.delete("/volumes/"+volumeID, nil, nil)
	ensureReaderClosed(resp)
	return err
}
