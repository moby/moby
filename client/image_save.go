package client

import (
	"context"
	"io"
	"net/url"
)

type ImageSaveResult interface {
	io.ReadCloser
}

// ImageSave retrieves one or more images from the docker host as an
// [ImageSaveResult]. Callers should close the reader, but the underlying
// [io.ReadCloser] is automatically closed if the context is canceled,
//
// Platforms is an optional parameter that specifies the platforms to save
// from the image. Passing a platform only has an effect if the input image
// is a multi-platform image.
func (cli *Client) ImageSave(ctx context.Context, imageIDs []string, saveOpts ...ImageSaveOption) (ImageSaveResult, error) {
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
		if err := cli.requiresVersion(ctx, "1.48", "platform"); err != nil {
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
	return &imageSaveResult{
		ReadCloser: newCancelReadCloser(ctx, resp.Body),
	}, nil
}

type imageSaveResult struct {
	io.ReadCloser
}

var (
	_ io.ReadCloser   = (*imageSaveResult)(nil)
	_ ImageSaveResult = (*imageSaveResult)(nil)
)
