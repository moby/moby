package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/image"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(ctx context.Context, imageID string, historyOpts ...ImageHistoryOption) ([]image.HistoryResponseItem, error) {
	query := url.Values{}

	var opts imageHistoryOpts
	for _, o := range historyOpts {
		if err := o.ApplyImageHistoryOption(&opts); err != nil {
			return nil, err
		}
	}

	if opts.apiOptions.Platform != nil {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return nil, err
		}

		p, err := encodePlatform(opts.apiOptions.Platform)
		if err != nil {
			return nil, err
		}
		query.Set("platform", p)
	}

	resp, err := cli.get(ctx, "/images/"+imageID+"/history", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var history []image.HistoryResponseItem
	err = json.NewDecoder(resp.Body).Decode(&history)
	return history, err
}
