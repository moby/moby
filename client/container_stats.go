package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/url"

	"github.com/docker/docker/api/types/container"
)

// ContainerStats returns near realtime stats for a given container.
// It's up to the caller to close the io.ReadCloser returned.
func (cli *Client) ContainerStats(ctx context.Context, containerID string, stream bool) (container.Stats, error) {
	query := url.Values{}
	query.Set("stream", "0")
	if stream {
		query.Set("stream", "1")
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/stats", query, nil)
	if err != nil {
		return container.Stats{}, err
	}

	osType := getDockerOS(resp.header.Get("Server"))
	return container.Stats{Body: resp.body, OSType: osType}, err
}
