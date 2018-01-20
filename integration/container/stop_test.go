package container

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/util/request"
	"github.com/gotestyourself/gotestyourself/icmd"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/require"
)

func TestDeleteDevicemapper(t *testing.T) {
	skip.IfCondition(t, testEnv.DaemonInfo.Driver != "devicemapper")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	foo, err := client.ContainerCreate(ctx,
		&container.Config{
			Cmd:   []string{"echo"},
			Image: "busybox",
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"foo",
	)
	require.NoError(t, err)

	err = client.ContainerStart(ctx, foo.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	inspect, err := client.ContainerInspect(ctx, foo.ID)
	require.NoError(t, err)

	poll.WaitOn(t, containerIsStopped(ctx, client, foo.ID), poll.WithDelay(100*time.Millisecond))

	deviceID := inspect.GraphDriver.Data["DeviceId"]

	// Find pool name from device name
	deviceName := inspect.GraphDriver.Data["DeviceName"]
	devicePrefix := deviceName[:strings.LastIndex(deviceName, "-")]
	devicePool := fmt.Sprintf("/dev/mapper/%s-pool", devicePrefix)

	result := icmd.RunCommand("dmsetup", "message", devicePool, "0", fmt.Sprintf("delete %s", deviceID))
	result.Assert(t, icmd.Success)

	err = client.ContainerRemove(ctx, foo.ID, types.ContainerRemoveOptions{})
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
