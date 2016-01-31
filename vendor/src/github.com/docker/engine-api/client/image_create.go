package client

import (
	"io"
	"net/url"

	"github.com/docker/engine-api/types"
)

// ImageCreate creates a new image based in the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(options types.ImageCreateOptions) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("fromImage", options.Parent)
	query.Set("tag", options.Tag)
	resp, err := cli.tryImageCreate(query, options.RegistryAuth)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) tryImageCreate(query url.Values, registryAuth string) (*serverResponse, error) {
	headers := map[string][]string{"X-Registry-Auth": {registryAuth}}
	return cli.post("/images/create", query, nil, headers)
}
