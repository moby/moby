package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client/imagehistory"
	"github.com/moby/moby/client/internal/opts"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(ctx context.Context, imageID string, historyOpts ...imagehistory.Option) ([]image.HistoryResponseItem, error) {
	query := url.Values{}

	var opts opts.ImageHistoryOptions
	for _, o := range historyOpts {
		if err := o.ApplyImageHistoryOption(ctx, &opts); err != nil {
			return nil, err
		}
	}

	if opts.ApiOptions.Platform != nil {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return nil, err
		}

		p, err := encodePlatform(opts.ApiOptions.Platform)
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
