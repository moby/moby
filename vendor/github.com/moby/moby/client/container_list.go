package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/versions"
)

// ContainerList returns the list of containers in the docker host.
func (cli *Client) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
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
		// Make sure we negotiated (if the client is configured to do so),
		// as code below contains API-version specific handling of options.
		//
		// Normally, version-negotiation (if enabled) would not happen until
		// the API request is made.
		if err := cli.checkVersion(ctx); err != nil {
			return nil, err
		}

		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return nil, err
		}
		if cli.version != "" && versions.LessThan(cli.version, "1.22") {
			legacyFormat, err := encodeLegacyFilters(filterJSON)
			if err != nil {
				return nil, err
			}
			filterJSON = legacyFormat
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
