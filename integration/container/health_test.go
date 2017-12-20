package container

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/util/request"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/stretchr/testify/require"
)

// TestHealthCheckWorkdir verifies that health-checks inherit the containers'
// working-dir.
func TestHealthCheckWorkdir(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	c, err := client.ContainerCreate(ctx,
		&container.Config{
			Image:      "busybox",
			Tty:        true,
			WorkingDir: "/foo",
			Cmd:        strslice.StrSlice([]string{"top"}),
			Healthcheck: &container.HealthConfig{
				Test:     []string{"CMD-SHELL", "if [ \"$PWD\" = \"/foo\" ]; then exit 0; else exit 1; fi;"},
				Interval: 50 * time.Millisecond,
				Retries:  3,
			},
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"healthtest",
	)
	require.NoError(t, err)
	err = client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	poll.WaitOn(t, pollForHealthStatus(ctx, client, c.ID, types.Healthy), poll.WithDelay(100*time.Millisecond))
}

func pollForHealthStatus(ctx context.Context, client client.APIClient, containerID string, healthStatus string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := client.ContainerInspect(ctx, containerID)

		switch {
		case err != nil:
			return poll.Error(err)
		case inspect.State.Health.Status == healthStatus:
			return poll.Success()
		default:
			return poll.Continue("waiting for container to become %s", healthStatus)
		}
	}
}
