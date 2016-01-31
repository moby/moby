package client

import (
	"io"
	"net/url"

	"github.com/docker/engine-api/types"
)

// ImageImport creates a new image based in the source options.
// It returns the JSON content in the response body.
func (cli *Client) ImageImport(options types.ImageImportOptions) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("fromSrc", options.SourceName)
	query.Set("repo", options.RepositoryName)
	query.Set("tag", options.Tag)
	query.Set("message", options.Message)
	for _, change := range options.Changes {
		query.Add("changes", change)
	}

	resp, err := cli.postRaw("/images/create", query, options.Source, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
