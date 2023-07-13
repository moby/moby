package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
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
	t.Cleanup(setupTest(t))
	client := testEnv.APIClient()
	ctx := context.Background()

	testData := []struct {
		doc                         string
		devices                     []string
		isolation                   containertypes.Isolation
		expectedStartFailure        bool
		expectedStartFailureMessage string
		expectedExitCode            int
		expectedStdout              string
		expectedStderr              string
	}{
		{
			doc:              "process/no device mounted",
			isolation:        containertypes.IsolationProcess,
			expectedExitCode: 1,
		},
		{
			doc:            "process/class/5B45201D-F2F2-4F3B-85BB-30FF1F953599 mounted",
			devices:        []string{"class/5B45201D-F2F2-4F3B-85BB-30FF1F953599"},
			isolation:      containertypes.IsolationProcess,
			expectedStdout: "/Windows/System32/HostDriverStore/FileRepository",
		},
		{
			doc:            "process/class://5B45201D-F2F2-4F3B-85BB-30FF1F953599 mounted",
			devices:        []string{"class://5B45201D-F2F2-4F3B-85BB-30FF1F953599"},
			isolation:      containertypes.IsolationProcess,
			expectedStdout: "/Windows/System32/HostDriverStore/FileRepository",
		},
		{
			doc:            "process/vpci-class-guid://5B45201D-F2F2-4F3B-85BB-30FF1F953599 mounted",
			devices:        []string{"vpci-class-guid://5B45201D-F2F2-4F3B-85BB-30FF1F953599"},
			isolation:      containertypes.IsolationProcess,
			expectedStdout: "/Windows/System32/HostDriverStore/FileRepository",
		},
		{
			doc:              "hyperv/no device mounted",
			isolation:        containertypes.IsolationHyperV,
			expectedExitCode: 1,
		},
		{
			doc:                         "hyperv/class/5B45201D-F2F2-4F3B-85BB-30FF1F953599 mounted",
			devices:                     []string{"class/5B45201D-F2F2-4F3B-85BB-30FF1F953599"},
			isolation:                   containertypes.IsolationHyperV,
			expectedStartFailure:        !testEnv.RuntimeIsWindowsContainerd(),
			expectedStartFailureMessage: "device assignment is not supported for HyperV containers",
			expectedStdout:              "/Windows/System32/HostDriverStore/FileRepository",
		},
		{
			doc:                         "hyperv/class://5B45201D-F2F2-4F3B-85BB-30FF1F953599 mounted",
			devices:                     []string{"class://5B45201D-F2F2-4F3B-85BB-30FF1F953599"},
			isolation:                   containertypes.IsolationHyperV,
			expectedStartFailure:        !testEnv.RuntimeIsWindowsContainerd(),
			expectedStartFailureMessage: "device assignment is not supported for HyperV containers",
			expectedStdout:              "/Windows/System32/HostDriverStore/FileRepository",
		},
		{
			doc:                         "hyperv/vpci-class-guid://5B45201D-F2F2-4F3B-85BB-30FF1F953599 mounted",
			devices:                     []string{"vpci-class-guid://5B45201D-F2F2-4F3B-85BB-30FF1F953599"},
			isolation:                   containertypes.IsolationHyperV,
			expectedStartFailure:        !testEnv.RuntimeIsWindowsContainerd(),
			expectedStartFailureMessage: "device assignment is not supported for HyperV containers",
			expectedStdout:              "/Windows/System32/HostDriverStore/FileRepository",
		},
	}

	for _, d := range testData {
		d := d
		t.Run(d.doc, func(t *testing.T) {
			t.Parallel()
			deviceOptions := []func(*container.TestContainerConfig){container.WithIsolation(d.isolation)}
			for _, deviceName := range d.devices {
				deviceOptions = append(deviceOptions, container.WithWindowsDevice(deviceName))
			}

			id := container.Create(ctx, t, client, deviceOptions...)

			// Hyper-V isolation is failing even with no actual devices added.
			// TODO: Once https://github.com/moby/moby/issues/43395 is resolved,
			// remove this skip.If and validate the expected behaviour under Hyper-V.
			skip.If(t, d.isolation == containertypes.IsolationHyperV && !d.expectedStartFailure, "FIXME. HyperV isolation setup is probably incorrect in the test")

			err := client.ContainerStart(ctx, id, types.ContainerStartOptions{})
			if d.expectedStartFailure {
				assert.ErrorContains(t, err, d.expectedStartFailureMessage)
				return
			}

			assert.NilError(t, err)

			poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))

			// /Windows/System32/HostDriverStore is mounted from the host when class GUID 5B45201D-F2F2-4F3B-85BB-30FF1F953599
			// is mounted. See `C:\windows\System32\containers\devices.def` on a Windows host for (slightly more) details.
			res, err := container.Exec(ctx, client, id, []string{
				"sh", "-c",
				"ls -d /Windows/System32/HostDriverStore/* | grep /Windows/System32/HostDriverStore/FileRepository",
			})
			assert.NilError(t, err)
			assert.Equal(t, d.expectedExitCode, res.ExitCode)
			if d.expectedExitCode == 0 {
				assert.Equal(t, d.expectedStdout, strings.TrimSpace(res.Stdout()))
				assert.Equal(t, d.expectedStderr, strings.TrimSpace(res.Stderr()))
			}
		})
	}
}
