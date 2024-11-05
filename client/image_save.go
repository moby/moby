package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/url"

	"github.com/docker/docker/api/types/image"
)

// ImageSave retrieves one or more images from the docker host as an io.ReadCloser.
// It's up to the caller to store the images and close the stream.
func (cli *Client) ImageSave(ctx context.Context, imageIDs []string, opts image.SaveOptions) (io.ReadCloser, error) {
	query := url.Values{
		"names": imageIDs,
	}

	if opts.Platform != nil {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return nil, err
		}

		p, err := encodePlatform(opts.Platform)
		if err != nil {
			return nil, err
		}
		query.Set("platform", p)
	}

	resp, err := cli.get(ctx, "/images/get", query, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
