package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client/pkg/versions"
)

// ImageList returns a list of images in the docker host.
//
// Experimental: Set the [image.ListOptions.Manifest] option
// to include [image.Summary.Manifests] with information about image manifests.
// This is experimental and might change in the future without any backward
// compatibility.
func (cli *Client) ImageList(ctx context.Context, options ImageListOptions) (ImageListResult, error) {
	var images []image.Summary

	query := url.Values{}

	options.Filters.updateURLValues(query)
	if options.All {
		query.Set("all", "1")
	}
	if options.SharedSize {
		query.Set("shared-size", "1")
	}
	if options.Manifests {
		// Make sure we negotiated (if the client is configured to do so),
		// as code below contains API-version specific handling of options.
		//
		// Normally, version-negotiation (if enabled) would not happen until
		// the API request is made.
		if err := cli.checkVersion(ctx); err != nil {
			return ImageListResult{}, err
		}

		if versions.GreaterThanOrEqualTo(cli.version, "1.47") {
			query.Set("manifests", "1")
		}
	}

	resp, err := cli.get(ctx, "/images/json", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ImageListResult{}, err
	}

	err = json.NewDecoder(resp.Body).Decode(&images)
	return ImageListResult{Items: images}, err
}
