package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// ImageLoad loads an image in the docker host from the client host. It's up
// to the caller to close the [io.ReadCloser] in the [ImageLoadResult.Body]
// returned by this function.
//
// If the context is canceled, the underlying [io.ReadCloser] is automatically
// closed.
func (cli *Client) ImageLoad(ctx context.Context, input io.Reader, loadOpts ...ImageLoadOption) (ImageLoadResult, error) {
	var opts imageLoadOpts
	for _, opt := range loadOpts {
		if err := opt.Apply(&opts); err != nil {
			return ImageLoadResult{}, err
		}
	}

	query := url.Values{}
	query.Set("quiet", "0")
	if opts.apiOptions.Quiet {
		query.Set("quiet", "1")
	}
	if len(opts.apiOptions.Platforms) > 0 {
		if err := cli.requiresVersion(ctx, "1.48", "platform"); err != nil {
			return ImageLoadResult{}, err
		}

		p, err := encodePlatforms(opts.apiOptions.Platforms...)
		if err != nil {
			return ImageLoadResult{}, err
		}
		query["platform"] = p
	}

	resp, err := cli.postRaw(ctx, "/images/load", query, input, http.Header{
		"Content-Type": {"application/x-tar"},
	})
	if err != nil {
		return ImageLoadResult{}, err
	}
	return ImageLoadResult{
		Body: newCancelReadCloser(ctx, resp.Body),
	}, nil
}

// ImageLoadResult returns information to the client about a load process.
type ImageLoadResult struct {
	// Body must be closed to avoid a resource leak
	Body io.ReadCloser
}
