package container // import "github.com/moby/moby/integration/container"

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

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
		err := client.ContainerStop(ctx, name, nil)
		assert.NilError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsStopped(ctx, client, name), poll.WithDelay(100*time.Millisecond))
	}
}
