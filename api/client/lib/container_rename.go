package lib

import "net/url"

// ContainerRename changes the name of a given container.
func (cli *Client) ContainerRename(containerID, newContainerName string) error {
	var query url.Values
	query.Set("name", newContainerName)
	resp, err := cli.POST("/containers/"+containerID+"/rename", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
