package lib

import "net/url"

// ContainerRemoveOptions holds parameters to remove containers.
type ContainerRemoveOptions struct {
	ContainerID   string
	RemoveVolumes bool
	RemoveLinks   bool
	Force         bool
}

// ContainerRemove kills and removes a container from the docker host.
func (cli *Client) ContainerRemove(options ContainerRemoveOptions) error {
	var query url.Values
	if options.RemoveVolumes {
		query.Set("v", "1")
	}
	if options.RemoveLinks {
		query.Set("link", "1")
	}

	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.DELETE("/containers/"+options.ContainerID, query, nil)
	ensureReaderClosed(resp)
	return err
}
