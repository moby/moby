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
	client := testEnv.APIClient()
	ctx := context.Background()

	names := []string{"verifyRestart1-" + t.Name(), "verifyRestart2-" + t.Name()}
	for _, name := range names {
		container.Run(ctx, t, client,
			container.WithName(name),
			container.WithCmd("false"),
			container.WithRestartPolicy("always"),
		)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsInState(ctx, client, name, "running", "restarting"), poll.WithDelay(100*time.Millisecond))
	}

	for _, name := range names {
		err := client.ContainerStop(ctx, name, containertypes.StopOptions{})
		assert.NilError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsStopped(ctx, client, name), poll.WithDelay(100*time.Millisecond))
	}
}
