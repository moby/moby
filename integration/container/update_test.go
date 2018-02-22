package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRestartPolicy(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithCmd("sh", "-c", "sleep 1 && false"), func(c *container.TestContainerConfig) {
		c.HostConfig.RestartPolicy = containertypes.RestartPolicy{
			Name:              "on-failure",
			MaximumRetryCount: 3,
		}
	})

	_, err := client.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
		RestartPolicy: containertypes.RestartPolicy{
			Name:              "on-failure",
			MaximumRetryCount: 5,
		},
	})
	require.NoError(t, err)

	timeout := 60 * time.Second
	if testEnv.OSType == "windows" {
		timeout = 180 * time.Second
	}

	poll.WaitOn(t, containerIsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond), poll.WithTimeout(timeout))

	inspect, err := client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, inspect.RestartCount, 5)
	assert.Equal(t, inspect.HostConfig.RestartPolicy.MaximumRetryCount, 5)
}

func TestUpdateRestartWithAutoRemove(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
		c.HostConfig.AutoRemove = true
	})

	_, err := client.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
		RestartPolicy: containertypes.RestartPolicy{
			Name: "always",
		},
	})
	testutil.ErrorContains(t, err, "Restart policy cannot be updated because AutoRemove is enabled for the container")
}
