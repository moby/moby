package lib

import (
	"io"
	"net/url"
)

// ImageLoad loads an image in the docker host from the client host.
// It's up to the caller to close the io.ReadCloser returned by
// this function.
func (cli *Client) ImageLoad(input io.Reader) (io.ReadCloser, error) {
	resp, err := cli.postRaw("/images/load", url.Values{}, input, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
