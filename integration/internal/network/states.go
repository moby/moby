package network

import (
	"context"

	"gotest.tools/v3/poll"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// IsRemoved verifies the network is removed.
func IsRemoved(ctx context.Context, client client.NetworkAPIClient, networkID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		_, err := client.NetworkInspect(ctx, networkID, network.InspectOptions{})
		if err == nil {
			return poll.Continue("waiting for network %s to be removed", networkID)
		}
		return poll.Success()
	}
}
