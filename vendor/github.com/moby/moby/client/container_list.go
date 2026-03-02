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
	Limit   int
	Filters Filters

	// Latest is non-functional and should not be used. Use Limit: 1 instead.
	//
	// Deprecated: the Latest option is non-functional and should not be used. Use Limit: 1 instead.
	Latest bool

	// Since is no longer supported. Use the "since" filter instead.
	//
	// Deprecated: the Since option is no longer supported since docker 1.12 (API 1.24). Use the "since" filter instead.
	Since string

	// Before is no longer supported. Use the "since" filter instead.
	//
	// Deprecated: the Before option is no longer supported since docker 1.12 (API 1.24). Use the "before" filter instead.
	Before string
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

	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
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
