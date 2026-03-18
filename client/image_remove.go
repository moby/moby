package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/image"
)

// ImageRemove removes an image from the docker host.
func (cli *Client) ImageRemove(ctx context.Context, imageID string, options ImageRemoveOptions) (ImageRemoveResult, error) {
	query := url.Values{}

	if options.Force {
		query.Set("force", "1")
	}
	if !options.PruneChildren {
		query.Set("noprune", "1")
	}

	if len(options.Platforms) > 0 {
		p, err := encodePlatforms(options.Platforms...)
		if err != nil {
			return ImageRemoveResult{}, err
		}
		query["platforms"] = p
	}

	resp, err := cli.delete(ctx, "/images/"+imageID, query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ImageRemoveResult{}, err
	}

	var dels []image.DeleteResponse
	err = json.NewDecoder(resp.Body).Decode(&dels)
	return ImageRemoveResult{Items: dels}, err
}
