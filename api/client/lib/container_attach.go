package lib

import (
	"net/url"

	"github.com/docker/docker/api/types"
)

// ContainerAttach attaches a connection to a container in the server.
// It returns a types.HijackedConnection with the hijacked connection
// and the a reader to get output. It's up to the called to close
// the hijacked connection by calling types.HijackedResponse.Close.
func (cli *Client) ContainerAttach(options types.ContainerAttachOptions) (*types.HijackedResponse, error) {
	query := url.Values{}
	if options.Stream {
		query.Set("stream", "1")
	}
	if options.Stdin {
		query.Set("stdin", "1")
	}
	if options.Stdout {
		query.Set("stdout", "1")
	}
	if options.Stderr {
		query.Set("stderr", "1")
	}

	headers := map[string][]string{"Content-Type": {"text/plain"}}
	return cli.postHijacked("/containers/"+options.ContainerID+"/attach", query, nil, headers)
}
