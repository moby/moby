package lib

import (
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/parsers/filters"
)

// ContainerListOptions holds parameters to list containers with.
type ContainerListOptions struct {
	Quiet  bool
	Size   bool
	All    bool
	Latest bool
	Since  string
	Before string
	Limit  int
	Filter filters.Args
}

// ContainerList returns the list of containers in the docker host.
func (cli *Client) ContainerList(options ContainerListOptions) ([]types.Container, error) {
	var query url.Values

	if options.All {
		query.Set("all", "1")
	}

	if options.Limit != -1 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}

	if options.Since != "" {
		query.Set("since", options.Since)
	}

	if options.Before != "" {
		query.Set("before", options.Before)
	}

	if options.Size {
		query.Set("size", "1")
	}

	if options.Filter.Len() > 0 {
		filterJSON, err := filters.ToParam(options.Filter)
		if err != nil {
			return nil, err
		}

		query.Set("filters", filterJSON)
	}

	resp, err := cli.GET("/containers/json", query, nil)
	if err != nil {
		return nil, err
	}
	defer ensureReaderClosed(resp)

	var containers []types.Container
	err = json.NewDecoder(resp.body).Decode(&containers)
	return containers, err
}
