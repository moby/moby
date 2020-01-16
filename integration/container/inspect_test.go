package container // import "github.com/moby/moby/integration/container"

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/moby/moby/integration/internal/container"
	"github.com/moby/moby/testutil/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

func TestInspectCpusetInConfigPre120(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" || !testEnv.DaemonInfo.CPUSet)

	defer setupTest(t)()
	client := request.NewAPIClient(t, client.WithVersion("1.19"))
	ctx := context.Background()

	name := "cpusetinconfig-pre120-" + t.Name()
	// Create container with up to-date-API
	container.Run(ctx, t, request.NewAPIClient(t), container.WithName(name),
		container.WithCmd("true"),
		func(c *container.TestContainerConfig) {
			c.HostConfig.Resources.CpusetCpus = "0"
		},
	)
	poll.WaitOn(t, container.IsInState(ctx, client, name, "exited"), poll.WithDelay(100*time.Millisecond))

	_, body, err := client.ContainerInspectWithRaw(ctx, name, false)
	assert.NilError(t, err)

	var inspectJSON map[string]interface{}
	err = json.Unmarshal(body, &inspectJSON)
	assert.NilError(t, err, "unable to unmarshal body for version 1.19: %s", err)

	config, ok := inspectJSON["Config"]
	assert.Check(t, is.Equal(true, ok), "Unable to find 'Config'")

	cfg := config.(map[string]interface{})
	_, ok = cfg["Cpuset"]
	assert.Check(t, is.Equal(true, ok), "API version 1.19 expected to include Cpuset in 'Config'")
}
