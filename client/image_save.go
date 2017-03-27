package client

import (
	"encoding/json"
	"io"
	"net/url"

	"github.com/docker/docker/api/types"

	"golang.org/x/net/context"
)

// ImageSave retrieves one or more images from the docker host as an io.ReadCloser.
// It's up to the caller to store the images and close the stream.
func (cli *Client) ImageSave(ctx context.Context, images []string, opts types.ImageSaveOptions) (io.ReadCloser, error) {
	query := url.Values{
		"names": images,
	}
	query.Set("format", opts.Format)
	refsJSON, err := json.Marshal(opts.Refs)
	if err != nil {
		return nil, err
	}
	query.Set("refs", string(refsJSON))

	resp, err := cli.get(ctx, "/images/get", query, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
