package lib

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/parsers/filters"
)

// ImageListOptions holds parameters to filter the list of images with.
type ImageListOptions struct {
	MatchName string
	All       bool
	Filters   filters.Args
}

// ImageList returns a list of images in the docker host.
func (cli *Client) ImageList(options ImageListOptions) ([]types.Image, error) {
	var (
		images []types.Image
		query  url.Values
	)

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToParam(options.Filters)
		if err != nil {
			return images, err
		}
		query.Set("filters", filterJSON)
	}
	if options.MatchName != "" {
		// FIXME rename this parameter, to not be confused with the filters flag
		query.Set("filter", options.MatchName)
	}
	if options.All {
		query.Set("all", "1")
	}

	serverResp, err := cli.GET("/images/json?", query, nil)
	if err != nil {
		return images, err
	}
	defer ensureReaderClosed(serverResp)

	err = json.NewDecoder(serverResp.body).Decode(&images)
	return images, err
}
