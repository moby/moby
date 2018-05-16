package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	containertypes "github.com/docker/docker/api/types/container"
)

// CheckpointListOptions holds parameters to list checkpoints for a container
type CheckpointListOptions struct {
	CheckpointDir string
}

// CheckpointList returns the checkpoints of the given container in the docker host
func (cli *Client) CheckpointList(ctx context.Context, container string, options CheckpointListOptions) ([]containertypes.Checkpoint, error) {
	var checkpoints []containertypes.Checkpoint

	query := url.Values{}
	if options.CheckpointDir != "" {
		query.Set("dir", options.CheckpointDir)
	}

	resp, err := cli.get(ctx, "/containers/"+container+"/checkpoints", query, nil)
	if err != nil {
		return checkpoints, wrapResponseError(err, resp, "container", container)
	}

	err = json.NewDecoder(resp.body).Decode(&checkpoints)
	ensureReaderClosed(resp)
	return checkpoints, err
}
