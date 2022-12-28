package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

// hcs can sometimes take a long time to stop container.
const StopContainerWindowsPollTimeout = 75 * time.Second

func TestStopContainerWithRestartPolicyAlways(t *testing.T) {
	defer setupTest(t)()
	apiClient := testEnv.APIClient()
	ctx := context.Background()

	names := []string{"verifyRestart1-" + t.Name(), "verifyRestart2-" + t.Name()}
	for _, name := range names {
		container.Run(ctx, t, apiClient,
			container.WithName(name),
			container.WithCmd("false"),
			container.WithRestartPolicy(containertypes.RestartPolicyAlways),
		)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsInState(ctx, apiClient, name, "running", "restarting"), poll.WithDelay(100*time.Millisecond))
	}

	for _, name := range names {
		err := apiClient.ContainerStop(ctx, name, containertypes.StopOptions{})
		assert.NilError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsStopped(ctx, apiClient, name), poll.WithDelay(100*time.Millisecond))
	}
}
