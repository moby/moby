package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// ImageLoad loads an image in the docker host from the client host.
// It's up to the caller to close the [io.ReadCloser] in the
// [ImageLoadResult] returned by this function.
//
// Platform is an optional parameter that specifies the platform to load from
// the provided multi-platform image. Passing a platform only has an effect
// if the input image is a multi-platform image.
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
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
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
		body: resp.Body,
	}, nil
}

// ImageLoadResult returns information to the client about a load process.
type ImageLoadResult struct {
	// Body must be closed to avoid a resource leak
	body io.ReadCloser
}

func (r ImageLoadResult) Read(p []byte) (n int, err error) {
	return r.body.Read(p)
}

func (r ImageLoadResult) Close() error {
	if r.body == nil {
		return nil
	}
	return r.body.Close()
}
