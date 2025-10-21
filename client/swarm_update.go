package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmUpdateOptions contains options for updating a swarm.
type SwarmUpdateOptions struct {
	Swarm                  swarm.Spec
	RotateWorkerToken      bool
	RotateManagerToken     bool
	RotateManagerUnlockKey bool
}

// SwarmUpdateResult represents the result of a SwarmUpdate operation.
type SwarmUpdateResult struct{}

// SwarmUpdate updates the swarm.
func (cli *Client) SwarmUpdate(ctx context.Context, version swarm.Version, options SwarmUpdateOptions) (SwarmUpdateResult, error) {
	query := url.Values{}
	query.Set("version", version.String())
	query.Set("rotateWorkerToken", strconv.FormatBool(options.RotateWorkerToken))
	query.Set("rotateManagerToken", strconv.FormatBool(options.RotateManagerToken))
	query.Set("rotateManagerUnlockKey", strconv.FormatBool(options.RotateManagerUnlockKey))
	resp, err := cli.post(ctx, "/swarm/update", query, options.Swarm, nil)
	defer ensureReaderClosed(resp)
	return SwarmUpdateResult{}, err
}
