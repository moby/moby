package client

import (
	"net/url"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
	"os"
)

// ContainerRemove kills and removes a container from the docker host.
func (cli *Client) ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error {
	query := url.Values{}
	if options.RemoveVolumes {
		query.Set("v", "1")
	}
	if options.RemoveLinks {
		query.Set("link", "1")
	}

	if options.Force {
		query.Set("force", "1")
	}

	// check and remove cidfile directly
	removeCidFile(cli, ctx, containerID)

	resp, err := cli.delete(ctx, "/containers/"+containerID, query, nil)
	ensureReaderClosed(resp)
	return err
}

func removeCidFile(cli *Client, ctx context.Context, containerID string) {
	cj, _ := cli.ContainerInspect(ctx, containerID)

	if cj.ContainerJSONBase != nil && cj.HostConfig != nil {
		cidfile := cj.HostConfig.ContainerIDFile
		if len(cidfile) > 0 {
			os.Remove(cidfile)
		}
	}
}
