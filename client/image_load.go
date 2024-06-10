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
func (cli *Client) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (image.LoadResponse, error) {
	v := url.Values{}
	v.Set("quiet", "0")
	if quiet {
		v.Set("quiet", "1")
	}
	resp, err := cli.postRaw(ctx, "/images/load", v, input, http.Header{
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
