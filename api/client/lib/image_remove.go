package lib

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
)

// ImageRemove removes an image from the docker host.
func (cli *Client) ImageRemove(options types.ImageRemoveOptions) ([]types.ImageDelete, error) {
	query := url.Values{}

	if options.Force {
		query.Set("force", "1")
	}
	if !options.PruneChildren {
		query.Set("noprune", "1")
	}

	resp, err := cli.DELETE("/images/"+options.ImageID, query, nil)
	if err != nil {
		return nil, err
	}
	defer ensureReaderClosed(resp)

	var dels []types.ImageDelete
	err = json.NewDecoder(resp.body).Decode(&dels)
	return dels, err
}
