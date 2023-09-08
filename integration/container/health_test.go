package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
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
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	defer setupTest(t)()
	ctx := context.Background()
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithTty(true), container.WithWorkingDir("/foo"), func(c *container.TestContainerConfig) {
		c.Config.Healthcheck = &containertypes.HealthConfig{
			Test:     []string{"CMD-SHELL", "if [ \"$PWD\" = \"/foo\" ]; then exit 0; else exit 1; fi;"},
			Interval: 50 * time.Millisecond,
			Retries:  3,
		}
	})

	poll.WaitOn(t, pollForHealthStatus(ctx, apiClient, cID, types.Healthy), poll.WithDelay(100*time.Millisecond))
}

// GitHub #37263
// Do not stop healthchecks just because we sent a signal to the container
func TestHealthKillContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "Windows only supports SIGKILL and SIGTERM? See https://github.com/moby/moby/issues/39574")
	defer setupTest(t)()

	ctx := context.Background()
	apiClient := testEnv.APIClient()

	id := container.Run(ctx, t, apiClient, func(c *container.TestContainerConfig) {
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
	poll.WaitOn(t, pollForHealthStatus(ctxPoll, apiClient, id, "healthy"), poll.WithDelay(100*time.Millisecond))

	err := apiClient.ContainerKill(ctx, id, "SIGUSR1")
	assert.NilError(t, err)

	ctxPoll, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	poll.WaitOn(t, pollForHealthStatus(ctxPoll, apiClient, id, "unhealthy"), poll.WithDelay(100*time.Millisecond))

	err = apiClient.ContainerKill(ctx, id, "SIGUSR1")
	assert.NilError(t, err)

	ctxPoll, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	poll.WaitOn(t, pollForHealthStatus(ctxPoll, apiClient, id, "healthy"), poll.WithDelay(100*time.Millisecond))
}

// TestHealthCheckProcessKilled verifies that health-checks exec get killed on time-out.
func TestHealthCheckProcessKilled(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.Config.Healthcheck = &containertypes.HealthConfig{
			Test:     []string{"CMD", "sh", "-c", `echo "logs logs logs"; sleep 60`},
			Interval: 100 * time.Millisecond,
			Timeout:  50 * time.Millisecond,
			Retries:  1,
		}
	})
	poll.WaitOn(t, pollForHealthCheckLog(ctx, apiClient, cID, "Health check exceeded timeout (50ms): logs logs logs\n"))
}

func TestHealthStartInterval(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "The shell commands used in the test healthcheck do not work on Windows")
	defer setupTest(t)()
	ctx := context.Background()
	apiClient := testEnv.APIClient()

	// Note: Windows is much slower than linux so this use longer intervals/timeouts
	id := container.Run(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.Config.Healthcheck = &containertypes.HealthConfig{
			Test:          []string{"CMD-SHELL", `count="$(cat /tmp/health)"; if [ -z "${count}" ]; then let count=0; fi; let count=${count}+1; echo -n ${count} | tee /tmp/health; if [ ${count} -lt 3 ]; then exit 1; fi`},
			Interval:      30 * time.Second,
			StartInterval: time.Second,
			StartPeriod:   30 * time.Second,
		}
	})

	ctxPoll, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dl, _ := ctxPoll.Deadline()

	poll.WaitOn(t, func(log poll.LogT) poll.Result {
		if ctxPoll.Err() != nil {
			return poll.Error(ctxPoll.Err())
		}
		inspect, err := apiClient.ContainerInspect(ctxPoll, id)
		if err != nil {
			return poll.Error(err)
		}
		if inspect.State.Health.Status != "healthy" {
			if len(inspect.State.Health.Log) > 0 {
				t.Log(inspect.State.Health.Log[len(inspect.State.Health.Log)-1])
			}
			return poll.Continue("waiting on container to be ready")
		}
		return poll.Success()
	}, poll.WithDelay(100*time.Millisecond), poll.WithTimeout(time.Until(dl)))
	cancel()

	ctxPoll, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	dl, _ = ctxPoll.Deadline()

	poll.WaitOn(t, func(log poll.LogT) poll.Result {
		inspect, err := apiClient.ContainerInspect(ctxPoll, id)
		if err != nil {
			return poll.Error(err)
		}

		hLen := len(inspect.State.Health.Log)
		if hLen < 2 {
			return poll.Continue("waiting for more healthcheck results")
		}

		h1 := inspect.State.Health.Log[hLen-1]
		h2 := inspect.State.Health.Log[hLen-2]
		if h1.Start.Sub(h2.Start) >= inspect.Config.Healthcheck.Interval {
			return poll.Success()
		}
		t.Log(h1.Start.Sub(h2.Start))
		return poll.Continue("waiting for health check interval to switch from the start interval")
	}, poll.WithDelay(time.Second), poll.WithTimeout(time.Until(dl)))
}

func pollForHealthCheckLog(ctx context.Context, client client.APIClient, containerID string, expected string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := client.ContainerInspect(ctx, containerID)
		if err != nil {
			return poll.Error(err)
		}
		healthChecksTotal := len(inspect.State.Health.Log)
		if healthChecksTotal > 0 {
			output := inspect.State.Health.Log[healthChecksTotal-1].Output
			if output == expected {
				return poll.Success()
			}
			return poll.Error(fmt.Errorf("expected %q, got %q", expected, output))
		}
		return poll.Continue("waiting for container healthcheck logs")
	}
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
