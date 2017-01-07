package client

import (
	"encoding/json"
	"errors"
	"net/url"

	"strings"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// ImageRemove removes an image from the docker host.
func (cli *Client) ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDelete, error) {
	if strings.TrimSpace(imageID) == "" {
		return nil, errors.New("image name cannot be blank")
	}

	if _, err := parseNamed(imageID); err != nil {
		return nil, err
	}

	query := url.Values{}

	if options.Force {
		query.Set("force", "1")
	}
	if !options.PruneChildren {
		query.Set("noprune", "1")
	}

	resp, err := cli.delete(ctx, "/images/"+imageID, query, nil)
	if err != nil {
		return nil, err
	}

	var dels []types.ImageDelete
	err = json.NewDecoder(resp.body).Decode(&dels)
	ensureReaderClosed(resp)
	return dels, err
}
