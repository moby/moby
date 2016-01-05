package lib

import (
	"net/url"

	"github.com/docker/engine-api/types"
)

// ImageTag tags an image in the docker host
func (cli *Client) ImageTag(options types.ImageTagOptions) error {
	query := url.Values{}
	query.Set("repo", options.RepositoryName)
	query.Set("tag", options.Tag)
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.post("/images/"+options.ImageID+"/tag", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
