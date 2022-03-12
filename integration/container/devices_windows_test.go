package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// TestWindowsDevices that Windows Devices are correctly propagated
// via HostConfig.Devices through to the implementation in hcsshim.
func TestWindowsDevices(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "windows")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	testData := []struct {
		doc              string
		devices          []string
		expectedExitCode int
		expectedStdout   string
		expectedStderr   string
	}{
		{
			doc:              "no device mounted",
			expectedExitCode: 1,
		},
		{
			doc:            "class/5B45201D-F2F2-4F3B-85BB-30FF1F953599 mounted",
			devices:        []string{"class/5B45201D-F2F2-4F3B-85BB-30FF1F953599"},
			expectedStdout: "/Windows/System32/HostDriverStore/FileRepository",
		},
	}

	for _, d := range testData {
		d := d
		t.Run(d.doc, func(t *testing.T) {
			t.Parallel()
			deviceOptions := []func(*container.TestContainerConfig){container.WithIsolation(containertypes.IsolationProcess)}
			for _, deviceName := range d.devices {
				deviceOptions = append(deviceOptions, container.WithWindowsDevice(deviceName))
			}
			id := container.Run(ctx, t, client, deviceOptions...)

			poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))

			// /Windows/System32/HostDriverStore is mounted from the host when class GUID 5B45201D-F2F2-4F3B-85BB-30FF1F953599
			// is mounted. See `C:\windows\System32\containers\devices.def` on a Windows host for (slightly more) details.
			res, err := container.Exec(ctx, client, id, []string{"sh", "-c",
				"ls -d /Windows/System32/HostDriverStore/* | grep /Windows/System32/HostDriverStore/FileRepository"})
			assert.NilError(t, err)
			assert.Equal(t, d.expectedExitCode, res.ExitCode)
			if d.expectedExitCode == 0 {
				assert.Equal(t, d.expectedStdout, strings.TrimSpace(res.Stdout()))
				assert.Equal(t, d.expectedStderr, strings.TrimSpace(res.Stderr()))
			}

		})
	}
}
