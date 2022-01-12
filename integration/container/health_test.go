package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// TestHealthCheckWorkdir verifies that health-checks inherit the containers'
// working-dir.
func TestHealthCheckWorkdir(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "FIXME")
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client, container.WithTty(true), container.WithWorkingDir("/foo"), func(c *container.TestContainerConfig) {
		c.Config.Healthcheck = &containertypes.HealthConfig{
			Test:     []string{"CMD-SHELL", "if [ \"$PWD\" = \"/foo\" ]; then exit 0; else exit 1; fi;"},
			Interval: 50 * time.Millisecond,
			Retries:  3,
		}
	})

	poll.WaitOn(t, pollForHealthStatus(ctx, client, cID, types.Healthy), poll.WithDelay(100*time.Millisecond))
}

// GitHub #37263
// Do not stop healthchecks just because we sent a signal to the container
func TestHealthKillContainer(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "Windows only supports SIGKILL and SIGTERM? See https://github.com/moby/moby/issues/39574")
	defer setupTest(t)()

	ctx := context.Background()
	client := testEnv.APIClient()

	id := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		cmd := `
# Set the initial HEALTH value so the healthcheck passes
HEALTH="1"
echo $HEALTH > /health

# Any time doHealth is run we flip the value
# This lets us use kill signals to determine when healtchecks have run.
doHealth() {
	case "$HEALTH" in
		"0")
			HEALTH="1"
			;;
		"1")
			HEALTH="0"
			;;
	esac
	echo $HEALTH > /health
}

trap 'doHealth' USR1

while true; do sleep 1; done
`
		c.Config.Cmd = []string{"/bin/sh", "-c", cmd}
		c.Config.Healthcheck = &containertypes.HealthConfig{
			Test:     []string{"CMD-SHELL", `[ "$(cat /health)" = "1" ]`},
			Interval: time.Second,
			Retries:  5,
		}
	})

	ctxPoll, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	poll.WaitOn(t, pollForHealthStatus(ctxPoll, client, id, "healthy"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerKill(ctx, id, "SIGUSR1")
	assert.NilError(t, err)

	ctxPoll, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	poll.WaitOn(t, pollForHealthStatus(ctxPoll, client, id, "unhealthy"), poll.WithDelay(100*time.Millisecond))

	err = client.ContainerKill(ctx, id, "SIGUSR1")
	assert.NilError(t, err)

	ctxPoll, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	poll.WaitOn(t, pollForHealthStatus(ctxPoll, client, id, "healthy"), poll.WithDelay(100*time.Millisecond))
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
