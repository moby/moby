package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/image"
)

// ImageRemoveOptions holds parameters to remove images.
type ImageRemoveOptions struct {
	Force         bool
	PruneChildren bool
}

// ImageRemove removes an image from the docker host.
func (cli *Client) ImageRemove(ctx context.Context, imageID string, options ImageRemoveOptions) ([]image.DeleteResponseItem, error) {
	query := url.Values{}

	if options.Force {
		query.Set("force", "1")
	}
	if !options.PruneChildren {
		query.Set("noprune", "1")
	}

	var dels []image.DeleteResponseItem
	resp, err := cli.delete(ctx, "/images/"+imageID, query, nil)
	if err != nil {
		return dels, wrapResponseError(err, resp, "image", imageID)
	}

	err = json.NewDecoder(resp.body).Decode(&dels)
	ensureReaderClosed(resp)
	return dels, err
}
