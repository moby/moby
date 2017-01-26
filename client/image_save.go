package client

import (
	"io"
	"net/url"

	"golang.org/x/net/context"
)

// ImageSave retrieves one or more images from the docker host as an io.ReadCloser.
// It's up to the caller to store the images and close the stream.
func (cli *Client) ImageSave(ctx context.Context, names []string) (io.ReadCloser, error) {
	for _, name := range names {
		if _, err := parseNamed(name); err != nil {
			return nil, err
		}
	}

	query := url.Values{
		"names": names,
	}

	resp, err := cli.get(ctx, "/images/get", query, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
