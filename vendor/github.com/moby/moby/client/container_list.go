package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
)

// ContainerListOptions holds parameters to list containers with.
type ContainerListOptions struct {
	Size    bool
	All     bool
	Latest  bool
	Since   string
	Before  string
	Limit   int
	Filters filters.Args
}

// ContainerList returns the list of containers in the docker host.
func (cli *Client) ContainerList(ctx context.Context, options ContainerListOptions) ([]container.Summary, error) {
	query := url.Values{}

	if options.All {
		query.Set("all", "1")
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

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return nil, err
		}
		query.Set("filters", filterJSON)
	}

	resp, err := cli.get(ctx, "/containers/json", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var containers []container.Summary
	err = json.NewDecoder(resp.Body).Decode(&containers)
	return containers, err
}
