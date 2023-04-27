package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDeleteDevicemapper(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.Driver != "devicemapper")
	skip.If(t, testEnv.IsRemoteDaemon)

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	id := container.Run(ctx, t, client, container.WithName("foo-"+t.Name()), container.WithCmd("echo"))

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
