package client

import (
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"golang.org/x/net/context"
)

// ContainerStats returns near realtime stats for a given container.
// It's up to the caller to close the io.ReadCloser returned.
func (cli *Client) ContainerStats(ctx context.Context, containerID string, stream bool) (types.ContainerStats, error) {
	query := url.Values{}
	query.Set("stream", "0")
	if stream {
		query.Set("stream", "1")
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/stats", query, nil)
	if err != nil {
		return types.ContainerStats{}, err
	}

	osType := getDockerOS(resp.header.Get("Server"))
	return types.ContainerStats{Body: resp.body, OSType: osType}, err
}

// ContainerStatsAll returns near realtime stats for all containers.
// It's up to the caller to close the io.ReadCloser returned.
func (cli *Client) ContainerStatsAll(ctx context.Context, options types.StatsAllOptions) (types.ContainerStats, error) {
	query := url.Values{}
	query.Set("stream", "0")
	if options.Stream {
		query.Set("stream", "1")
	}

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToParamWithVersion(cli.version, options.Filters)

		if err != nil {
			return types.ContainerStats{}, err
		}

		query.Set("filters", filterJSON)
	}

	resp, err := cli.get(ctx, "/containers/-/stats", query, nil)
	if err != nil {
		return types.ContainerStats{}, err
	}
	osType := getDockerOS(resp.header.Get("Server"))
	return types.ContainerStats{Body: resp.body, OSType: osType}, err
}
