package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/icmd"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestStopContainerWithRestartPolicyAlways(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	names := []string{"verifyRestart1-" + t.Name(), "verifyRestart2-" + t.Name()}
	for _, name := range names {
		container.Run(t, ctx, client, container.WithName(name), container.WithCmd("false"), func(c *container.TestContainerConfig) {
			c.HostConfig.RestartPolicy.Name = "always"
		})
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsInState(ctx, client, name, "running", "restarting"), poll.WithDelay(100*time.Millisecond))
	}

	for _, name := range names {
		err := client.ContainerStop(ctx, name, nil)
		assert.NilError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsStopped(ctx, client, name), poll.WithDelay(100*time.Millisecond))
	}
}

func TestDeleteDevicemapper(t *testing.T) {
	skip.IfCondition(t, testEnv.DaemonInfo.Driver != "devicemapper")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	id := container.Run(t, ctx, client, container.WithName("foo-"+t.Name()), container.WithCmd("echo"))

	poll.WaitOn(t, container.IsStopped(ctx, client, id), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, id)
	assert.NilError(t, err)

	deviceID := inspect.GraphDriver.Data["DeviceId"]

	// Find pool name from device name
	deviceName := inspect.GraphDriver.Data["DeviceName"]
	devicePrefix := deviceName[:strings.LastIndex(deviceName, "-")]
	devicePool := fmt.Sprintf("/dev/mapper/%s-pool", devicePrefix)

	result := icmd.RunCommand("dmsetup", "message", devicePool, "0", fmt.Sprintf("delete %s", deviceID))
	result.Assert(t, icmd.Success)

	err = client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{})
	assert.NilError(t, err)
}
