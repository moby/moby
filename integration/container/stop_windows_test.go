package container // import "github.com/docker/docker/integration/container"

import (
	"strconv"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// TestStopContainerWithTimeout checks that ContainerStop with
// a timeout works as documented, i.e. in case of negative timeout
// waiting is not limited (issue #35311).
func TestStopContainerWithTimeout(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

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
			1, 0x40010004,
		},
		{
			"too small timeout: expect forceful container kill",
			2, 0x40010004,
		},
		{
			"big enough timeout: expect graceful container stop",
			120, 42,
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
			ctx := testutil.StartSpan(ctx, t)
			id := container.Run(ctx, t, apiClient, testCmd)

			err := apiClient.ContainerStop(ctx, id, containertypes.StopOptions{Timeout: &d.timeout})
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsStopped(ctx, apiClient, id))

			inspect, err := apiClient.ContainerInspect(ctx, id)
			assert.NilError(t, err)
			assert.Equal(t, inspect.State.ExitCode, d.expectedExitCode)
		})
	}
}
