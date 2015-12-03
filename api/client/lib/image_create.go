package lib

import (
	"io"
	"net/url"
)

// CreateImageOptions holds information to create images.
type CreateImageOptions struct {
	// Parent is the image to create this image from
	Parent string
	// Tag is the name to tag this image
	Tag string
	// RegistryAuth is the base64 encoded credentials for this server
	RegistryAuth string
}

// CreateImage creates a new image based in the parent options.
// It returns the JSON content in the response body.
func (cli *Client) CreateImage(options CreateImageOptions) (io.ReadCloser, error) {
	var query url.Values
	query.Set("fromImage", options.Parent)
	query.Set("tag", options.Tag)

	headers := map[string][]string{"X-Registry-Auth": {options.RegistryAuth}}
	resp, err := cli.POST("/images/create", query, nil, headers)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
