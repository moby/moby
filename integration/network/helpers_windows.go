package network

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"gotest.tools/v3/assert/cmp"
)

// DeleteInterface deletes a network interface
func DeleteInterface(_ context.Context, _ *testing.T, _ string) cmp.Comparison {
	panic("DeleteInterface is not implemented on Windows")
}

// IsNetworkAvailable provides a comparison to check if a docker network is available
func IsNetworkAvailable(ctx context.Context, c client.NetworkAPIClient, name string) cmp.Comparison {
	return func() cmp.Result {
		networks, err := c.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return cmp.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return cmp.ResultSuccess
			}
		}
		return cmp.ResultFailure(fmt.Sprintf("could not find network %s", name))
	}
}

// IsNetworkNotAvailable provides a comparison to check if a docker network is not available
func IsNetworkNotAvailable(ctx context.Context, c client.NetworkAPIClient, name string) cmp.Comparison {
	return func() cmp.Result {
		networks, err := c.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return cmp.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return cmp.ResultFailure(fmt.Sprintf("network %s is still present", name))
			}
		}
		return cmp.ResultSuccess
	}
}
