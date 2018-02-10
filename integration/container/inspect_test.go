package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectCpusetInConfigPre120(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux" || !testEnv.DaemonInfo.CPUSet)

	defer setupTest(t)()
	client := request.NewAPIClient(t, client.WithVersion("1.19"))
	ctx := context.Background()

	name := "cpusetinconfig-pre120"
	// Create container with up to-date-API
	container.Run(t, ctx, request.NewAPIClient(t), container.WithName(name),
		container.WithCmd("true"),
		func(c *container.TestContainerConfig) {
			c.HostConfig.Resources.CpusetCpus = "0"
		},
	)
	poll.WaitOn(t, containerIsInState(ctx, client, name, "exited"), poll.WithDelay(100*time.Millisecond))

	_, body, err := client.ContainerInspectWithRaw(ctx, name, false)
	require.NoError(t, err)

	var inspectJSON map[string]interface{}
	err = json.Unmarshal(body, &inspectJSON)
	require.NoError(t, err, "unable to unmarshal body for version 1.19: %s", err)

	config, ok := inspectJSON["Config"]
	assert.Equal(t, ok, true, "Unable to find 'Config'")

	cfg := config.(map[string]interface{})
	_, ok = cfg["Cpuset"]
	assert.Equal(t, ok, true, "API version 1.19 expected to include Cpuset in 'Config'")
}
