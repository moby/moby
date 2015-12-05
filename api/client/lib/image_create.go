package lib

import (
	"io"
	"net/url"

	"github.com/docker/docker/api/types"
)

// ImageCreate creates a new image based in the parent options.
// It returns the JSON content in the response body.
func (cli *Client) ImageCreate(options types.ImageCreateOptions) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("fromImage", options.Parent)
	query.Set("tag", options.Tag)

	headers := map[string][]string{"X-Registry-Auth": {options.RegistryAuth}}
	resp, err := cli.POST("/images/create", query, nil, headers)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
