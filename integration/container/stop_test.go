package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	"gotest.tools/icmd"
	"gotest.tools/poll"
	"gotest.tools/skip"
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

// TestStopContainerWithTimeout checks that ContainerStop with
// a timeout works as documented, i.e. in case of negative timeout
// waiting is not limited (issue #35311).
func TestStopContainerWithTimeout(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
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
			id := container.Run(t, ctx, client, testCmd)

			timeout := time.Duration(d.timeout) * time.Second
			err := client.ContainerStop(ctx, id, &timeout)
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
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")

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
