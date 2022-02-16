package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// TestStopContainerWithTimeout checks that ContainerStop with
// a timeout works as documented, i.e. in case of negative timeout
// waiting is not limited (issue #35311).
func TestStopContainerWithTimeout(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	testCmd := container.WithCmd("sh", "-c", "sleep 2 && exit 42")
	testData := []struct {
		doc              string
		timeout          int
		expectedExitCode int
	}{
		// In case container is forcefully killed, 137 is returned,
		// otherwise the exit code from the above script
		{
			"zero timeout: expect forceful container kill",
			0, 137,
		},
		{
			"too small timeout: expect forceful container kill",
			1, 137,
		},
		{
			"big enough timeout: expect graceful container stop",
			3, 42,
		},
		{
			"unlimited timeout: expect graceful container stop",
			-1, 42,
		},
	}

	for _, d := range testData {
		d := d
		t.Run(strconv.Itoa(d.timeout), func(t *testing.T) {
			t.Parallel()
			id := container.Run(ctx, t, client, testCmd)

			err := client.ContainerStop(ctx, id, containertypes.StopOptions{Timeout: &d.timeout})
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsStopped(ctx, client, id),
				poll.WithDelay(100*time.Millisecond))

			inspect, err := client.ContainerInspect(ctx, id)
			assert.NilError(t, err)
			assert.Equal(t, inspect.State.ExitCode, d.expectedExitCode)
		})
	}
}

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
