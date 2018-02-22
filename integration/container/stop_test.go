package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/gotestyourself/gotestyourself/icmd"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/require"
)

func TestStopContainerWithRestartPolicyAlways(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	names := []string{"verifyRestart1", "verifyRestart2"}
	for _, name := range names {
		container.Run(t, ctx, client, container.WithName(name), container.WithCmd("false"), func(c *container.TestContainerConfig) {
			c.HostConfig.RestartPolicy.Name = "always"
		})
	}

	for _, name := range names {
		poll.WaitOn(t, containerIsInState(ctx, client, name, "running", "restarting"), poll.WithDelay(100*time.Millisecond))
	}

	for _, name := range names {
		err := client.ContainerStop(ctx, name, nil)
		require.NoError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, containerIsStopped(ctx, client, name), poll.WithDelay(100*time.Millisecond))
	}
}

func TestDeleteDevicemapper(t *testing.T) {
	skip.IfCondition(t, testEnv.DaemonInfo.Driver != "devicemapper")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	id := container.Run(t, ctx, client, container.WithName("foo"), container.WithCmd("echo"))

	poll.WaitOn(t, containerIsStopped(ctx, client, id), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, id)
	require.NoError(t, err)

	deviceID := inspect.GraphDriver.Data["DeviceId"]

	// Find pool name from device name
	deviceName := inspect.GraphDriver.Data["DeviceName"]
	devicePrefix := deviceName[:strings.LastIndex(deviceName, "-")]
	devicePool := fmt.Sprintf("/dev/mapper/%s-pool", devicePrefix)

	result := icmd.RunCommand("dmsetup", "message", devicePool, "0", fmt.Sprintf("delete %s", deviceID))
	result.Assert(t, icmd.Success)

	err = client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{})
	require.NoError(t, err)
}

func containerIsStopped(ctx context.Context, client client.APIClient, containerID string) func(log poll.LogT) poll.Result {
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

func containerIsInState(ctx context.Context, client client.APIClient, containerID string, state ...string) func(log poll.LogT) poll.Result {
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
		return poll.Continue("waiting for container to be running, currently %s", inspect.State.Status)
	}
}
