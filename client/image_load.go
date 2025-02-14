package client // import "github.com/docker/docker/client"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageLoadOption is a type representing functional options for the image load operation.
type ImageLoadOption interface {
	Apply(*imageLoadOpts) error
}
type imageLoadOptionFunc func(opt *imageLoadOpts) error

func (f imageLoadOptionFunc) Apply(o *imageLoadOpts) error {
	return f(o)
}

type imageLoadOpts struct {
	apiOptions image.LoadOptions
}

// ImageLoadWithQuiet sets the quiet option for the image load operation.
func ImageLoadWithQuiet(quiet bool) ImageLoadOption {
	return imageLoadOptionFunc(func(opt *imageLoadOpts) error {
		opt.apiOptions.Quiet = quiet
		return nil
	})
}

// ImageLoadWithPlatforms sets the platforms to be loaded from the image.
func ImageLoadWithPlatforms(platforms ...ocispec.Platform) ImageLoadOption {
	return imageLoadOptionFunc(func(opt *imageLoadOpts) error {
		if opt.apiOptions.Platforms != nil {
			return fmt.Errorf("platforms already set to %v", opt.apiOptions.Platforms)
		}
		opt.apiOptions.Platforms = platforms
		return nil
	})
}

// ImageLoad loads an image in the docker host from the client host.
// It's up to the caller to close the io.ReadCloser in the
// ImageLoadResponse returned by this function.
//
// Platform is an optional parameter that specifies the platform to load from
// the provided multi-platform image. This is only has effect if the input image
// is a multi-platform image.
func (cli *Client) ImageLoad(ctx context.Context, input io.Reader, loadOpts ...ImageLoadOption) (image.LoadResponse, error) {
	var opts imageLoadOpts
	for _, opt := range loadOpts {
		if err := opt.Apply(&opts); err != nil {
			return image.LoadResponse{}, err
		}
	}

	query := url.Values{}
	query.Set("quiet", "0")
	if opts.apiOptions.Quiet {
		query.Set("quiet", "1")
	}
	if len(opts.apiOptions.Platforms) > 0 {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return image.LoadResponse{}, err
		}

		p, err := encodePlatforms(opts.apiOptions.Platforms...)
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
