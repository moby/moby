package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/versions"
)

// ImageList returns a list of images in the docker host.
func (cli *Client) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	// Make sure we negotiated (if the client is configured to do so),
	// as code below contains API-version specific handling of options.
	//
	// Normally, version-negotiation (if enabled) would not happen until
	// the API request is made.
	cli.checkVersion(ctx)

	var images []image.Summary
	query := url.Values{}

	optionFilters := options.Filters
	referenceFilters := optionFilters.Get("reference")
	if versions.LessThan(cli.version, "1.25") && len(referenceFilters) > 0 {
		query.Set("filter", referenceFilters[0])
		for _, filterValue := range referenceFilters {
			optionFilters.Del("reference", filterValue)
		}
	}
	if optionFilters.Len() > 0 {
		//nolint:staticcheck // ignore SA1019 for old code
		filterJSON, err := filters.ToParamWithVersion(cli.version, optionFilters)
		if err != nil {
			return images, err
		}
		query.Set("filters", filterJSON)
	}
	if options.All {
		query.Set("all", "1")
	}
	if options.SharedSize && versions.GreaterThanOrEqualTo(cli.version, "1.42") {
		query.Set("shared-size", "1")
	}

	serverResp, err := cli.get(ctx, "/images/json", query, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return images, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&images)
	return images, err
}
