package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/image"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(ctx context.Context, imageID string, opts image.HistoryOptions) ([]image.HistoryResponseItem, error) {
	query := url.Values{}
	if opts.Platform != nil {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return nil, err
		}

		p, err := encodePlatform(opts.Platform)
		if err != nil {
			return nil, err
		}
		query.Set("platform", p)
	}

	serverResp, err := cli.get(ctx, "/images/"+imageID+"/history", query, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return nil, err
	}

	var history []image.HistoryResponseItem
	err = json.NewDecoder(serverResp.body).Decode(&history)
	return history, err
}
