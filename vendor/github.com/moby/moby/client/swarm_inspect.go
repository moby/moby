package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmInspectOptions holds options for inspecting a swarm.
type SwarmInspectOptions struct {
	// Add future optional parameters here
}

// SwarmInspectResult represents the result of a SwarmInspect operation.
type SwarmInspectResult struct {
	Swarm swarm.Swarm
}

// SwarmInspect inspects the swarm.
func (cli *Client) SwarmInspect(ctx context.Context, options SwarmInspectOptions) (SwarmInspectResult, error) {
	resp, err := cli.get(ctx, "/swarm", nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SwarmInspectResult{}, err
	}

	var s swarm.Swarm
	err = json.NewDecoder(resp.Body).Decode(&s)
	return SwarmInspectResult{Swarm: s}, err
}
