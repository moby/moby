package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageHistoryWithPlatform sets the platform for the image history operation.
func ImageHistoryWithPlatform(platform ocispec.Platform) ImageHistoryOption {
	return imageHistoryOptionFunc(func(opt *imageHistoryOpts) error {
		if opt.apiOptions.Platform != nil {
			return fmt.Errorf("platform already set to %s", *opt.apiOptions.Platform)
		}
		opt.apiOptions.Platform = &platform
		return nil
	})
}

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(ctx context.Context, imageID string, historyOpts ...ImageHistoryOption) (ImageHistoryResult, error) {
	query := url.Values{}

	var opts imageHistoryOpts
	for _, o := range historyOpts {
		if err := o.Apply(&opts); err != nil {
			return ImageHistoryResult{}, err
		}
	}

	if opts.apiOptions.Platform != nil {
		if err := cli.requiresVersion(ctx, "1.48", "platform"); err != nil {
			return ImageHistoryResult{}, err
		}

		p, err := encodePlatform(opts.apiOptions.Platform)
		if err != nil {
			return ImageHistoryResult{}, err
		}
		query.Set("platform", p)
	}

	resp, err := cli.get(ctx, "/images/"+imageID+"/history", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ImageHistoryResult{}, err
	}

	var history ImageHistoryResult
	err = json.NewDecoder(resp.Body).Decode(&history.Items)
	return history, err
}
