package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/moby/moby/api/types/container"
)

// ContainerTopOptions defines options for container top operations.
type ContainerTopOptions struct {
	Arguments []string
}

// ContainerTopResult represents the result of a ContainerTop operation.
type ContainerTopResult struct {
	Processes [][]string
	Titles    []string
}

// ContainerTop shows process information from within a container.
func (cli *Client) ContainerTop(ctx context.Context, containerID string, options ContainerTopOptions) (ContainerTopResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerTopResult{}, err
	}

	query := url.Values{}
	if len(options.Arguments) > 0 {
		query.Set("ps_args", strings.Join(options.Arguments, " "))
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/top", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerTopResult{}, err
	}

	var response container.TopResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return ContainerTopResult{Processes: response.Processes, Titles: response.Titles}, err
}
