package container

import (
	"context"
	"strings"

	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"gotest.tools/v3/poll"
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

// IsSuccessful verifies state.Status == "exited" && state.ExitCode == 0
func IsSuccessful(ctx context.Context, client client.APIClient, containerID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := client.ContainerInspect(ctx, containerID)
		if err != nil {
			return poll.Error(err)
		}
		if inspect.State.Status == "exited" {
			if inspect.State.ExitCode == 0 {
				return poll.Success()
			}
			return poll.Error(errors.Errorf("expected exit code 0, got %d", inspect.State.ExitCode))
		}
		return poll.Continue("waiting for container to be \"exited\", currently %s", inspect.State.Status)
	}
}

// IsRemoved verifies the container has been removed
func IsRemoved(ctx context.Context, cli client.APIClient, containerID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			if client.IsErrNotFound(err) {
				return poll.Success()
			}
			return poll.Error(err)
		}
		return poll.Continue("waiting for container to be removed, currently %s", inspect.State.Status)
	}
}
