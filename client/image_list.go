package client

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"golang.org/x/net/context"
)

// ImageList returns a list of images in the docker host.
func (cli *Client) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	var images []types.ImageSummary
	query := url.Values{}

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToParamWithVersion(cli.version, options.Filters)
		if err != nil {
			return images, err
		}
		query.Set("filters", filterJSON)
	}
	if options.All {
		query.Set("all", "1")
	}

	serverResp, err := cli.get(ctx, "/images/json", query, nil)
	if err != nil {
		return images, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&images)
	ensureReaderClosed(serverResp)
	return images, err
}
