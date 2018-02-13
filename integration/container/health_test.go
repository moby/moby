package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/gotestyourself/gotestyourself/poll"
)

// TestHealthCheckWorkdir verifies that health-checks inherit the containers'
// working-dir.
func TestHealthCheckWorkdir(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cID := container.Run(t, ctx, client, container.WithTty(true), container.WithWorkingDir("/foo"), func(c *container.TestContainerConfig) {
		c.Config.Healthcheck = &containertypes.HealthConfig{
			Test:     []string{"CMD-SHELL", "if [ \"$PWD\" = \"/foo\" ]; then exit 0; else exit 1; fi;"},
			Interval: 50 * time.Millisecond,
			Retries:  3,
		}
	})

	poll.WaitOn(t, pollForHealthStatus(ctx, client, cID, types.Healthy), poll.WithDelay(100*time.Millisecond))
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
