package client // import "github.com/docker/docker/client"

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/docker/docker/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageSaveOption interface {
	Apply(*imageSaveOpts) error
}
type imageSaveOptionFunc func(opt *imageSaveOpts) error

func (f imageSaveOptionFunc) Apply(o *imageSaveOpts) error {
	return f(o)
}

func ImageSaveWithPlatforms(platforms ...ocispec.Platform) ImageSaveOption {
	return imageSaveOptionFunc(func(opt *imageSaveOpts) error {
		if opt.apiOptions.Platforms != nil {
			return fmt.Errorf("platforms already set to %v", opt.apiOptions.Platforms)
		}
		opt.apiOptions.Platforms = platforms
		return nil
	})
}

type imageSaveOpts struct {
	apiOptions image.SaveOptions
}

// ImageSave retrieves one or more images from the docker host as an io.ReadCloser.
//
// Platforms is an optional parameter that specifies the platforms to save from the image.
// This is only has effect if the input image is a multi-platform image.
func (cli *Client) ImageSave(ctx context.Context, imageIDs []string, saveOpts ...ImageSaveOption) (io.ReadCloser, error) {
	var opts imageSaveOpts
	for _, opt := range saveOpts {
		if err := opt.Apply(&opts); err != nil {
			return nil, err
		}
	}

	query := url.Values{
		"names": imageIDs,
	}

	if len(opts.apiOptions.Platforms) > 0 {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return nil, err
		}
		p, err := encodePlatforms(opts.apiOptions.Platforms...)
		if err != nil {
			return nil, err
		}
		query["platform"] = p
	}

	resp, err := cli.get(ctx, "/images/get", query, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
