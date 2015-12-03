package lib

import (
	"io"
	"net/url"
)

// ImportImageOptions holds information to import images from the client host.
type ImportImageOptions struct {
	// Source is the data to send to the server to create this image from
	Source io.Reader
	// Source is the name of the source to import this image from
	SourceName string
	// RepositoryName is the name of the repository to import this image
	RepositoryName string
	// Message is the message to tag the image with
	Message string
	// Tag is the name to tag this image
	Tag string
	// Changes are the raw changes to apply to the image
	Changes []string
}

// ImportImage creates a new image based in the source options.
// It returns the JSON content in the response body.
func (cli *Client) ImportImage(options ImportImageOptions) (io.ReadCloser, error) {
	var query url.Values
	query.Set("fromSrc", options.SourceName)
	query.Set("repo", options.RepositoryName)
	query.Set("tag", options.Tag)
	query.Set("message", options.Message)
	for _, change := range options.Changes {
		query.Add("changes", change)
	}

	resp, err := cli.POSTRaw("/images/create", query, options.Source, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
