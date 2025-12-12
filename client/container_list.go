package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/container"
)

// ContainerListOptions holds parameters to list containers with.
type ContainerListOptions struct {
	Size    bool
	All     bool
	Latest  bool
	Since   string
	Before  string
	Limit   int
	Filters Filters
}

type ContainerListResult struct {
	Items []container.Summary
}

// ContainerList returns the list of containers in the docker host.
func (cli *Client) ContainerList(ctx context.Context, options ContainerListOptions) (ContainerListResult, error) {
	query := url.Values{}

	if options.All {
		query.Set("all", "1")
	}

	if options.Latest {
		query.Set("limit", "1")
	}

	if options.Limit > 0 {
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

	options.Filters.updateURLValues(query)

	resp, err := cli.get(ctx, "/containers/json", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerListResult{}, err
	}

	var containers []container.Summary
	err = json.NewDecoder(resp.Body).Decode(&containers)
	return ContainerListResult{Items: containers}, err
}
