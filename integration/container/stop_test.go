package container

import (
	"net/http"
	"testing"
	"time"

	"github.com/moby/moby/api/types/common"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// hcs can sometimes take a long time to stop container.
const StopContainerWindowsPollTimeout = 75 * time.Second

func TestStopContainerWithRestartPolicyAlways(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	names := []string{"verifyRestart1-" + t.Name(), "verifyRestart2-" + t.Name()}
	for _, name := range names {
		container.Run(ctx, t, apiClient,
			container.WithName(name),
			container.WithCmd("false"),
			container.WithRestartPolicy(containertypes.RestartPolicyAlways),
		)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsInState(ctx, apiClient, name, containertypes.StateRunning, containertypes.StateRestarting))
	}

	for _, name := range names {
		_, err := apiClient.ContainerStop(ctx, name, client.ContainerStopOptions{})
		assert.NilError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsStopped(ctx, apiClient, name))
	}
}

// TestStopContainerWithTimeout checks that ContainerStop with
// a timeout works as documented, i.e. in case of negative timeout
// waiting is not limited (issue #35311).
func TestStopContainerWithTimeout(t *testing.T) {
	isWindows := testEnv.DaemonInfo.OSType == "windows"
	// TODO(vvoland): Make this work on Windows
	skip.If(t, isWindows)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	forcefulKillExitCode := 137
	if isWindows {
		forcefulKillExitCode = 0x40010004
	}

	testCmd := container.WithCmd("sh", "-c", "sleep 10 && exit 42")
	testData := []struct {
		doc              string
		timeout          int
		expectedExitCode int
	}{
		// In case container is forcefully killed, 137 is returned,
		// otherwise the exit code from the above script
		{
			doc:              "zero timeout: expect forceful container kill",
			expectedExitCode: forcefulKillExitCode,
			timeout:          0,
		},
		{
			doc:              "too small timeout: expect forceful container kill",
			expectedExitCode: forcefulKillExitCode,
			timeout:          2,
		},
		{
			doc:              "big enough timeout: expect graceful container stop",
			expectedExitCode: 42,
			timeout:          20, // longer than "sleep 10" cmd
		},
		{
			doc:              "unlimited timeout: expect graceful container stop",
			expectedExitCode: 42,
			timeout:          -1,
		},
	}

	var pollOpts []poll.SettingOp
	if isWindows {
		pollOpts = append(pollOpts, poll.WithTimeout(StopContainerWindowsPollTimeout))
	}

	for _, tc := range testData {
		t.Run(tc.doc, func(t *testing.T) {
			// TODO(vvoland): Investigate why it helps
			// t.Parallel()
			id := container.Run(ctx, t, apiClient, testCmd)

			_, err := apiClient.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &tc.timeout})
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsStopped(ctx, apiClient, id), pollOpts...)

			inspect, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
			assert.NilError(t, err)
			assert.Check(t, is.Equal(inspect.Container.State.ExitCode, tc.expectedExitCode))
		})
	}
}

func TestContainerAPIPostContainerStop(t *testing.T) {
	apiClient := testEnv.APIClient()
	ctx := setupTest(t)

	tests := []struct {
		testName            string
		id                  string
		expStatusCode       int
		expError            string
		checkContainerState bool
		expStateRunning     bool
	}{
		{
			testName:            "no error",
			id:                  container.Run(ctx, t, apiClient),
			expStatusCode:       http.StatusNoContent,
			checkContainerState: true,
			expStateRunning:     false,
		},
		{
			testName:            "container already stopped",
			id:                  container.Create(ctx, t, apiClient),
			expStatusCode:       http.StatusNotModified,
			checkContainerState: true,
			expStateRunning:     false,
		},
		{
			testName:            "no such container",
			id:                  "test1234",
			expStatusCode:       http.StatusNotFound,
			expError:            `No such container: test1234`,
			checkContainerState: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			res, _, err := request.Post(ctx, "/containers/"+tc.id+"/stop")

			assert.Equal(t, res.StatusCode, tc.expStatusCode)
			assert.NilError(t, err)

			if tc.expError != "" {
				var respErr common.ErrorResponse
				assert.NilError(t, request.ReadJSONResponse(res, &respErr))
				assert.ErrorContains(t, respErr, tc.expError)
			}

			if tc.checkContainerState {
				inspectRes := container.Inspect(ctx, t, apiClient, tc.id)
				assert.Equal(t, inspectRes.State.Running, tc.expStateRunning)
			}
		})
	}
}
