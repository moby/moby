package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmUpdateOptions contains options for updating a swarm.
type SwarmUpdateOptions struct {
	Version                swarm.Version
	Spec                   swarm.Spec
	RotateWorkerToken      bool
	RotateManagerToken     bool
	RotateManagerUnlockKey bool
}

// SwarmUpdateResult represents the result of a SwarmUpdate operation.
type SwarmUpdateResult struct{}

// SwarmUpdate updates the swarm.
func (cli *Client) SwarmUpdate(ctx context.Context, options SwarmUpdateOptions) (SwarmUpdateResult, error) {
	query := url.Values{}
	query.Set("version", options.Version.String())
	query.Set("rotateWorkerToken", strconv.FormatBool(options.RotateWorkerToken))
	query.Set("rotateManagerToken", strconv.FormatBool(options.RotateManagerToken))
	query.Set("rotateManagerUnlockKey", strconv.FormatBool(options.RotateManagerUnlockKey))
	resp, err := cli.post(ctx, "/swarm/update", query, options.Spec, nil)
	defer ensureReaderClosed(resp)
	return SwarmUpdateResult{}, err
}
