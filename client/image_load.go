package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/image"
)

// ImageLoad loads an image in the docker host from the client host.
// It's up to the caller to close the io.ReadCloser in the
// ImageLoadResponse returned by this function.
//
// Deprecated: use [Client.ImageLoadWithOptions] instead.
func (cli *Client) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (image.LoadResponse, error) {
	return cli.imageLoad(ctx, input, image.LoadOptions{Quiet: quiet})
}

// ImageLoadWithOptions loads an image in the docker host from the client host.
// It's up to the caller to close the io.ReadCloser in the
// ImageLoadResponse returned by this function.
//
// Platform is an optional parameter that specifies the platform to load from
// the provided multi-platform image. This is only has effect if the input image
// is a multi-platform image.
func (cli *Client) ImageLoadWithOptions(ctx context.Context, input io.Reader, opts image.LoadOptions) (image.LoadResponse, error) {
	return cli.imageLoad(ctx, input, opts)
}

func (cli *Client) imageLoad(ctx context.Context, input io.Reader, opts image.LoadOptions) (image.LoadResponse, error) {
	query := url.Values{}
	query.Set("quiet", "0")
	if opts.Quiet {
		query.Set("quiet", "1")
	}
	if len(opts.Platforms) > 0 {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return image.LoadResponse{}, err
		}

		p, err := encodePlatforms(opts.Platforms...)
		if err != nil {
			return image.LoadResponse{}, err
		}
		query["platform"] = p
	}

	resp, err := cli.postRaw(ctx, "/images/load", query, input, http.Header{
		"Content-Type": {"application/x-tar"},
	})
	if err != nil {
		return image.LoadResponse{}, err
	}
	return image.LoadResponse{
		Body: resp.body,
		JSON: resp.header.Get("Content-Type") == "application/json",
	}, nil
}
