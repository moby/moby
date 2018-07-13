package container

import (
	"context"
	"strings"

	"github.com/docker/docker/client"
	"gotest.tools/poll"
)

// IsStopped verifies the container is in stopped state.
func IsStopped(ctx context.Context, client client.APIClient, containerID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := client.ContainerInspect(ctx, containerID)

		switch {
		case err != nil:
			return poll.Error(err)
		case !inspect.State.Running:
			return poll.Success()
		default:
			return poll.Continue("waiting for container to be stopped")
		}
	}
}

// IsInState verifies the container is in one of the specified state, e.g., "running", "exited", etc.
func IsInState(ctx context.Context, client client.APIClient, containerID string, state ...string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := client.ContainerInspect(ctx, containerID)
		if err != nil {
			return poll.Error(err)
		}
		for _, v := range state {
			if inspect.State.Status == v {
				return poll.Success()
			}
		}
		return poll.Continue("waiting for container to be one of (%s), currently %s", strings.Join(state, ", "), inspect.State.Status)
	}
}
