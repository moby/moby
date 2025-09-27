package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client/handle"
)

// ImageRemove removes an image from the docker host.
func (cli *Client) ImageRemove(ctx context.Context, img handle.ImageHandle, options ImageRemoveOptions) ([]image.DeleteResponse, error) {
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
			return nil, err
		}
		query["platforms"] = p
	}

	rimg, err := img.ResolveImage(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := cli.delete(ctx, "/images/"+rimg.RefOrTruncatedID, query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var dels []image.DeleteResponse
	err = json.NewDecoder(resp.Body).Decode(&dels)
	return dels, err
}
