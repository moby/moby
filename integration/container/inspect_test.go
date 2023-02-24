package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestInspectCpusetInConfigPre120(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" || !testEnv.DaemonInfo.CPUSet)

	defer setupTest(t)()
	client := request.NewAPIClient(t, client.WithVersion("1.19"))
	ctx := context.Background()

	name := strings.ToLower(t.Name())
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

func TestInspectAnnotations(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	annotations := map[string]string{
		"hello": "world",
		"foo":   "bar",
	}

	name := strings.ToLower(t.Name())
	id := container.Create(ctx, t, client,
		container.WithName(name),
		container.WithCmd("true"),
		func(c *container.TestContainerConfig) {
			c.HostConfig.Annotations = annotations
		},
	)

	inspect, err := client.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(inspect.HostConfig.Annotations, annotations))
}
