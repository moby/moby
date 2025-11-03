package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// ImageLoadResult returns information to the client about a load process.
// It implements [io.ReadCloser] and must be closed to avoid a resource leak.
type ImageLoadResult interface {
	io.ReadCloser
}

// ImageLoad loads an image in the docker host from the client host. It's up
// to the caller to close the [ImageLoadResult] returned by this function.
func (cli *Client) ImageLoad(ctx context.Context, input io.Reader, loadOpts ...ImageLoadOption) (ImageLoadResult, error) {
	var opts imageLoadOpts
	for _, opt := range loadOpts {
		if err := opt.Apply(&opts); err != nil {
			return nil, err
		}
	}

	query := url.Values{}
	query.Set("quiet", "0")
	if opts.apiOptions.Quiet {
		query.Set("quiet", "1")
	}
	if len(opts.apiOptions.Platforms) > 0 {
		if err := cli.requiresVersion(ctx, "1.48", "platform"); err != nil {
			return nil, err
		}

		p, err := encodePlatforms(opts.apiOptions.Platforms...)
		if err != nil {
			return nil, err
		}
		query["platform"] = p
	}

	resp, err := cli.postRaw(ctx, "/images/load", query, input, http.Header{
		"Content-Type": {"application/x-tar"},
	})
	if err != nil {
		return nil, err
	}
	return &imageLoadResult{
		body: resp.Body,
	}, nil
}

// imageLoadResult returns information to the client about a load process.
type imageLoadResult struct {
	// body must be closed to avoid a resource leak
	body io.ReadCloser
}

var (
	_ io.ReadCloser   = (*imageLoadResult)(nil)
	_ ImageLoadResult = (*imageLoadResult)(nil)
)

func (r *imageLoadResult) Read(p []byte) (int, error) {
	if r == nil || r.body == nil {
		return 0, io.EOF
	}
	return r.body.Read(p)
}

func (r *imageLoadResult) Close() error {
	if r == nil || r.body == nil {
		return nil
	}
	return r.body.Close()
}
